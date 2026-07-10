#!/usr/bin/env python3
"""Train the learned point reader (experiment-plan §8 step 8, dev-time only).

Input: one or more crop directories produced by `lazybg align -crops DIR`
(labels.csv + per-point PNGs). Each crop is one rectified board point; the
label is the ground-truth occupancy derived from the .mat transcription.

The task is a single 13-way classification per point crop:
    class 0        : empty
    class 1..6     : player A (CheckerA) with 1..5 checkers, 6 = "6 or more"
    class 7..12    : player B likewise

Split is BY GAME (never by crop) so near-duplicate frames of the same game
cannot leak between train and validation (experiment-plan §6).

Outputs (to --out):
    pointreader.pt     torch state_dict (input of export_go.py)
    pointreader.onnx   opset-17 ONNX, input float32 NCHW 3x160x32 in [0,1]
    pointreader.json   metadata: input size, class map, the flip rule
    report.txt         validation accuracy, per-class recall, confusion

Usage:
    uv venv --python 3.13 .venv
    uv pip install --python .venv/bin/python \
        --index-url https://download.pytorch.org/whl/cpu torch
    uv pip install --python .venv/bin/python onnx onnxruntime pillow numpy
    .venv/bin/python train_pointreader.py --crops ../corpus/crops/* --out out/
"""

import argparse
import csv
import json
import random
from collections import Counter
from pathlib import Path

import numpy as np
import torch
import torch.nn as nn
from PIL import Image

IN_W, IN_H = 32, 160  # preserves the ~1:5 point aspect
N_CLASSES = 13


def class_of(count: int, owner: str) -> int:
    if count == 0 or owner == "-":
        return 0
    c = min(count, 6)
    return c if owner == "A" else 6 + c


def class_name(k: int) -> str:
    if k == 0:
        return "empty"
    side = "A" if k <= 6 else "B"
    n = k if k <= 6 else k - 6
    return f"{side}{n}{'+' if n == 6 else ''}"


def load_rows(crop_dirs):
    rows = []
    for d in crop_dirs:
        d = Path(d)
        lp = d / "labels.csv"
        if not lp.exists():
            print(f"skip {d}: no labels.csv")
            continue
        with open(lp) as f:
            for r in csv.DictReader(f):
                rows.append(
                    {
                        "path": d / r["file"],
                        "rec": r["recording"],
                        "game": f'{r["recording"]}:{r["game"]}',
                        "point": int(r["point"]),
                        "cls": class_of(int(r["count"]), r["owner"]),
                    }
                )
    return rows


def load_image(row, train: bool):
    img = Image.open(row["path"]).convert("RGB")
    if row["point"] <= 12:
        # Bottom-half points stack from the bottom edge; flip so every crop
        # reads "checkers grow from the top" (matches pointreader.json rule).
        img = img.transpose(Image.FLIP_TOP_BOTTOM)
    if train:
        # light augmentation: jitter + sub-pixel-ish shifts via resized crop
        if random.random() < 0.5:
            img = img.transpose(Image.FLIP_LEFT_RIGHT)
        w, h = img.size
        dx, dy = random.randint(-3, 3), random.randint(-6, 6)
        img = img.crop((dx, dy, w + dx, h + dy))
    img = img.resize((IN_W, IN_H), Image.BILINEAR)
    x = np.asarray(img, dtype=np.float32) / 255.0
    if train:
        x = np.clip(x * random.uniform(0.8, 1.2) + random.uniform(-0.08, 0.08), 0, 1)
    return torch.from_numpy(x.transpose(2, 0, 1))  # CHW


class Batches:
    def __init__(self, rows, batch, train):
        self.rows, self.batch, self.train = rows, batch, train

    def __iter__(self):
        order = list(range(len(self.rows)))
        if self.train:
            random.shuffle(order)
        for i in range(0, len(order), self.batch):
            chunk = [self.rows[j] for j in order[i : i + self.batch]]
            x = torch.stack([load_image(r, self.train) for r in chunk])
            y = torch.tensor([r["cls"] for r in chunk], dtype=torch.long)
            yield x, y


class PointNet(nn.Module):
    """Tiny CNN — a few thousand crops, CPU inference in microseconds."""

    def __init__(self):
        super().__init__()
        def block(i, o):
            return [nn.Conv2d(i, o, 3, padding=1), nn.BatchNorm2d(o), nn.ReLU(), nn.MaxPool2d(2)]

        self.net = nn.Sequential(
            *block(3, 16), *block(16, 32), *block(32, 64), *block(64, 64),
            nn.AdaptiveAvgPool2d(1), nn.Flatten(),
            nn.Linear(64, 64), nn.ReLU(), nn.Dropout(0.2),
            nn.Linear(64, N_CLASSES),
        )

    def forward(self, x):
        return self.net(x)


def evaluate(model, rows, batch=128):
    model.eval()
    conf = np.zeros((N_CLASSES, N_CLASSES), dtype=int)
    with torch.no_grad():
        for x, y in Batches(rows, batch, train=False):
            pred = model(x).argmax(1)
            for t, p in zip(y.tolist(), pred.tolist()):
                conf[t][p] += 1
    acc = conf.trace() / max(conf.sum(), 1)
    return acc, conf


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--crops", nargs="+", required=True)
    ap.add_argument("--out", default="out")
    ap.add_argument("--epochs", type=int, default=30)
    ap.add_argument("--batch", type=int, default=64)
    ap.add_argument("--val-games", default="", help="comma list like rec:2,rec:5; default = every 3rd game")
    ap.add_argument("--val-recordings", default="", help="comma list of recording ids held out entirely (stronger than --val-games once several captures exist)")
    ap.add_argument("--exclude-recordings", default="", help="comma list of recording ids dropped entirely (e.g. duplicate manifests of the same video)")
    ap.add_argument("--seed", type=int, default=7)
    args = ap.parse_args()
    random.seed(args.seed)
    torch.manual_seed(args.seed)

    rows = load_rows(args.crops)
    if args.exclude_recordings:
        dropped = set(args.exclude_recordings.split(","))
        rows = [r for r in rows if r["rec"] not in dropped]
    if not rows:
        raise SystemExit("no labeled crops found")
    games = sorted({r["game"] for r in rows})
    if args.val_recordings:
        val_recs = set(args.val_recordings.split(","))
        train = [r for r in rows if r["rec"] not in val_recs]
        val = [r for r in rows if r["rec"] in val_recs]
        val_games = {g for g in games if g.split(":")[0] in val_recs}
    elif args.val_games:
        val_games = set(args.val_games.split(","))
        train = [r for r in rows if r["game"] not in val_games]
        val = [r for r in rows if r["game"] in val_games]
    else:
        val_games = set(games[::3])  # every third game held out
        train = [r for r in rows if r["game"] not in val_games]
        val = [r for r in rows if r["game"] in val_games]
    print(f"{len(rows)} crops, {len(games)} games -> train {len(train)} / val {len(val)} (val games: {sorted(val_games)})")
    print("train class counts:", dict(sorted(Counter(r["cls"] for r in train).items())))

    # Inverse-sqrt frequency class weights against the empty-point flood.
    counts = Counter(r["cls"] for r in train)
    weights = torch.tensor(
        [1.0 / (counts.get(k, 1) ** 0.5) for k in range(N_CLASSES)], dtype=torch.float32
    )
    weights /= weights.mean()

    model = PointNet()
    opt = torch.optim.Adam(model.parameters(), lr=1e-3)
    sched = torch.optim.lr_scheduler.CosineAnnealingLR(opt, T_max=args.epochs)
    lossf = nn.CrossEntropyLoss(weight=weights)

    best_acc, best_state = 0.0, None
    for epoch in range(args.epochs):
        model.train()
        tot, n = 0.0, 0
        for x, y in Batches(train, args.batch, train=True):
            opt.zero_grad()
            loss = lossf(model(x), y)
            loss.backward()
            opt.step()
            tot += loss.item() * len(y)
            n += len(y)
        sched.step()
        acc, _ = evaluate(model, val)
        star = ""
        if acc > best_acc:
            best_acc, best_state = acc, {k: v.clone() for k, v in model.state_dict().items()}
            star = " *"
        print(f"epoch {epoch+1:2d}: loss {tot/max(n,1):.4f}  val acc {acc:.4f}{star}")

    model.load_state_dict(best_state)
    acc, conf = evaluate(model, val)

    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=True)
    torch.save(model.state_dict(), out / "pointreader.pt")

    with open(out / "report.txt", "w") as f:
        f.write(f"val accuracy: {acc:.4f}  ({len(val)} crops, val games {sorted(val_games)})\n\n")
        f.write("per-class recall:\n")
        for k in range(N_CLASSES):
            tot = conf[k].sum()
            if tot:
                f.write(f"  {class_name(k):>6}: {conf[k][k]/tot:.3f}  ({conf[k][k]}/{tot})\n")
        f.write("\nconfusion (rows = truth):\n")
        hdr = " ".join(f"{class_name(k):>5}" for k in range(N_CLASSES))
        f.write(f"{'':>6} {hdr}\n")
        for k in range(N_CLASSES):
            f.write(f"{class_name(k):>6} " + " ".join(f"{v:>5}" for v in conf[k]) + "\n")
    print(open(out / "report.txt").read())

    # ONNX export + parity check.
    model.eval()
    dummy = torch.zeros(1, 3, IN_H, IN_W)
    onnx_path = out / "pointreader.onnx"
    torch.onnx.export(
        model, (dummy,), str(onnx_path),
        input_names=["crop"], output_names=["logits"],
        dynamic_axes={"crop": {0: "batch"}, "logits": {0: "batch"}},
        opset_version=17, dynamo=False,
    )
    import onnxruntime as ort

    sess = ort.InferenceSession(str(onnx_path), providers=["CPUExecutionProvider"])
    x = torch.rand(4, 3, IN_H, IN_W)
    with torch.no_grad():
        want = model(x).numpy()
    got = sess.run(None, {"crop": x.numpy()})[0]
    err = float(np.abs(got - want).max())
    print(f"onnx parity max |Δ| = {err:.2e}")
    assert err < 1e-4, "ONNX export diverges from torch"

    with open(out / "pointreader.json", "w") as f:
        json.dump(
            {
                "input": {"width": IN_W, "height": IN_H, "layout": "NCHW", "range": "[0,1]"},
                "classes": [class_name(k) for k in range(N_CLASSES)],
                "flip_rule": "vertically flip crops of points 1..12 before resize (stacks read top-down)",
                "val_accuracy": acc,
                "val_games": sorted(val_games),
                "train_crops": len(train),
            },
            f,
            indent=2,
        )
    print(f"wrote {onnx_path}")


if __name__ == "__main__":
    main()
