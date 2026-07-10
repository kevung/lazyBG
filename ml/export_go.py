#!/usr/bin/env python3
"""Export a trained PointNet to the pure-Go weight format (+ parity vectors).

The shipped app must stay a self-contained offline Go binary (CLAUDE.md §3
locked decision 10 — honest caveat noted there). For this first, tiny model
we sidestep the ONNX-Runtime shared library entirely: BatchNorms are folded
into their convolutions and the weights written to a flat little-endian
format that `internal/perceive/pointnet` executes directly. The ONNX file
stays the canonical interchange artifact.

Format "LZPN1" (all little-endian):
    magic  5 bytes "LZPN1"
    u32    tensor count
    per tensor:
        u32 name length, name bytes
        u32 ndims, u32 dims[ndims]
        f32 data (row-major)

Tensors: conv{i}.w [out,in,3,3], conv{i}.b [out] for i in 0..3;
         fc0.w [64,64], fc0.b [64]; fc1.w [13,64], fc1.b [13].

Also writes parity vectors: testvec.bin = one input (3x160x32 f32) followed
by the expected 13 logits, computed by the folded model.

Usage:
    python export_go.py --model out/pointreader.pt --out out/pointreader.bin
    python export_go.py --random-tiny --out testdata_fixture/   # Go test fixture
"""

import argparse
import struct
from pathlib import Path

import numpy as np
import torch
import torch.nn as nn

from train_pointreader import IN_H, IN_W, N_CLASSES, PointNet


def fold_bn(conv: nn.Conv2d, bn: nn.BatchNorm2d):
    """Return (w, b) with the BatchNorm folded into the convolution."""
    w = conv.weight.detach().numpy()
    b = conv.bias.detach().numpy() if conv.bias is not None else np.zeros(w.shape[0], np.float32)
    gamma = bn.weight.detach().numpy()
    beta = bn.bias.detach().numpy()
    mean = bn.running_mean.detach().numpy()
    var = bn.running_var.detach().numpy()
    scale = gamma / np.sqrt(var + bn.eps)
    return (w * scale[:, None, None, None]).astype(np.float32), (beta + (b - mean) * scale).astype(np.float32)


def tensors_of(model: PointNet):
    seq = model.net
    # layout: [conv bn relu pool] x4, gap, flatten, fc, relu, dropout, fc
    out = {}
    convs = [(seq[i], seq[i + 1]) for i in (0, 4, 8, 12)]
    for k, (conv, bn) in enumerate(convs):
        w, b = fold_bn(conv, bn)
        out[f"conv{k}.w"], out[f"conv{k}.b"] = w, b
    out["fc0.w"] = seq[18].weight.detach().numpy().astype(np.float32)
    out["fc0.b"] = seq[18].bias.detach().numpy().astype(np.float32)
    out["fc1.w"] = seq[21].weight.detach().numpy().astype(np.float32)
    out["fc1.b"] = seq[21].bias.detach().numpy().astype(np.float32)
    return out


def write_lzpn(path: Path, tensors):
    with open(path, "wb") as f:
        f.write(b"LZPN1")
        f.write(struct.pack("<I", len(tensors)))
        for name, t in tensors.items():
            nb = name.encode()
            f.write(struct.pack("<I", len(nb)))
            f.write(nb)
            f.write(struct.pack("<I", t.ndim))
            for d in t.shape:
                f.write(struct.pack("<I", d))
            f.write(np.ascontiguousarray(t, dtype="<f4").tobytes())


class Folded(nn.Module):
    """The inference-time network the Go code mirrors: conv+relu+pool x4,
    global average pool, fc+relu, fc."""

    def __init__(self, tensors):
        super().__init__()
        self.t = {k: torch.from_numpy(v.copy()) for k, v in tensors.items()}

    def forward(self, x):
        import torch.nn.functional as F

        for k in range(4):
            x = F.conv2d(x, self.t[f"conv{k}.w"], self.t[f"conv{k}.b"], padding=1)
            x = F.relu(x)
            x = F.max_pool2d(x, 2)
        x = x.mean(dim=(2, 3))
        x = F.relu(F.linear(x, self.t["fc0.w"], self.t["fc0.b"]))
        return F.linear(x, self.t["fc1.w"], self.t["fc1.b"])


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--model", help="state_dict .pt of a trained PointNet")
    ap.add_argument("--random-tiny", action="store_true", help="random-weights fixture for the Go parity test")
    ap.add_argument("--out", required=True)
    ap.add_argument("--seed", type=int, default=11)
    args = ap.parse_args()
    torch.manual_seed(args.seed)

    model = PointNet()
    if args.model:
        model.load_state_dict(torch.load(args.model, map_location="cpu", weights_only=True))
    elif not args.random_tiny:
        raise SystemExit("need --model or --random-tiny")
    model.eval()

    tensors = tensors_of(model)
    out = Path(args.out)
    if out.suffix == ".bin":
        out.parent.mkdir(parents=True, exist_ok=True)
        write_lzpn(out, tensors)
        print(f"wrote {out}")
        base = out.parent
    else:
        out.mkdir(parents=True, exist_ok=True)
        write_lzpn(out / "pointreader.bin", tensors)
        base = out
        print(f"wrote {out}/pointreader.bin")

    # Parity vectors from the folded reference (what Go must reproduce).
    x = torch.rand(1, 3, IN_H, IN_W)
    with torch.no_grad():
        want = Folded(tensors)(x).numpy()[0]
        # Sanity: folded must match the original net closely.
        orig = model(x).numpy()[0]
    err = float(np.abs(want - orig).max())
    print(f"fold parity max |Δ| = {err:.2e}")
    assert err < 1e-3, "BN folding diverged"

    with open(base / "testvec.bin", "wb") as f:
        f.write(struct.pack("<III", 3, IN_H, IN_W))
        f.write(np.ascontiguousarray(x.numpy(), dtype="<f4").tobytes())
        f.write(struct.pack("<I", N_CLASSES))
        f.write(np.ascontiguousarray(want, dtype="<f4").tobytes())
    print(f"wrote {base}/testvec.bin (expected logits: {np.round(want, 4)})")


if __name__ == "__main__":
    main()
