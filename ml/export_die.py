#!/usr/bin/env python3
"""Export a trained DieNet7 to the pure-Go LZPN1 weight format (+ parity).

Mirror of export_go.py for the die-value classifier (ml/train_dielabel.py):
BatchNorms folded into the convolutions, tensors conv0..2 + fc0/fc1 written
little-endian; dietestvec.bin holds one input (3x48x48) and the expected 7
logits for the Go parity test (internal/perceive/dienet).

Usage:
    ml/.venv/bin/python export_die.py --model out-dielabel-v3/dielabel.pt --out dievalue.bin
    ml/.venv/bin/python export_die.py --random-tiny --out ../testdata/dienet/
"""

import argparse
import struct
from pathlib import Path

import numpy as np
import torch
import torch.nn as nn

from train_dielabel import IN, DieNet7


def fold_bn(conv, bn):
    w = conv.weight.detach().numpy()
    b = conv.bias.detach().numpy() if conv.bias is not None else np.zeros(w.shape[0], np.float32)
    gamma = bn.weight.detach().numpy()
    beta = bn.bias.detach().numpy()
    mean = bn.running_mean.detach().numpy()
    var = bn.running_var.detach().numpy()
    scale = gamma / np.sqrt(var + bn.eps)
    return (w * scale[:, None, None, None]).astype(np.float32), (beta + (b - mean) * scale).astype(np.float32)


def tensors_of(model):
    seq = model.net
    # layout: [conv bn relu pool] x3, gap, flatten, fc, relu, dropout, fc
    out = {}
    for k, i in enumerate((0, 4, 8)):
        w, b = fold_bn(seq[i], seq[i + 1])
        out[f"conv{k}.w"], out[f"conv{k}.b"] = w, b
    out["fc0.w"] = seq[14].weight.detach().numpy().astype(np.float32)
    out["fc0.b"] = seq[14].bias.detach().numpy().astype(np.float32)
    out["fc1.w"] = seq[17].weight.detach().numpy().astype(np.float32)
    out["fc1.b"] = seq[17].bias.detach().numpy().astype(np.float32)
    return out


class Folded(nn.Module):
    """The BN-folded model, for computing parity logits."""

    def __init__(self, t):
        super().__init__()
        self.t = {k: torch.from_numpy(v.copy()) for k, v in t.items()}

    def forward(self, x):
        import torch.nn.functional as F
        for k in range(3):
            x = F.relu(F.conv2d(x, self.t[f"conv{k}.w"], self.t[f"conv{k}.b"], padding=1))
            x = F.max_pool2d(x, 2)
        x = x.mean(dim=(2, 3))
        x = F.relu(F.linear(x, self.t["fc0.w"], self.t["fc0.b"]))
        return F.linear(x, self.t["fc1.w"], self.t["fc1.b"])


def write_lzpn1(path, tensors):
    with open(path, "wb") as f:
        f.write(b"LZPN1")
        f.write(struct.pack("<I", len(tensors)))
        for name, arr in tensors.items():
            nb = name.encode()
            f.write(struct.pack("<I", len(nb)))
            f.write(nb)
            f.write(struct.pack("<I", arr.ndim))
            for d in arr.shape:
                f.write(struct.pack("<I", d))
            f.write(arr.astype("<f4").tobytes())


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--model")
    ap.add_argument("--random-tiny", action="store_true")
    ap.add_argument("--out", required=True)
    args = ap.parse_args()

    torch.manual_seed(3)
    model = DieNet7()
    if args.model:
        model.load_state_dict(torch.load(args.model, weights_only=True))
    elif not args.random_tiny:
        raise SystemExit("need --model or --random-tiny")
    model.eval()
    t = tensors_of(model)

    out = Path(args.out)
    if out.suffix == ".bin":
        bin_path, vec_path = out, out.parent / "dietestvec.bin"
    else:
        out.mkdir(parents=True, exist_ok=True)
        bin_path, vec_path = out / "dievalue.bin", out / "dietestvec.bin"
    write_lzpn1(bin_path, t)

    x = torch.rand(1, 3, IN, IN)
    with torch.no_grad():
        logits = Folded(t)(x)[0].numpy().astype(np.float32)
    with open(vec_path, "wb") as f:
        f.write(x[0].numpy().astype("<f4").tobytes())
        f.write(logits.astype("<f4").tobytes())
    print(f"wrote {bin_path} and {vec_path} (expected logits: {np.round(logits, 4)})")


if __name__ == "__main__":
    main()
