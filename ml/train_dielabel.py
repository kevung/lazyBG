#!/usr/bin/env python3
"""Train the die detector+reader on HAND-LABELED box crops (7 classes).

Input: a diceboxcrops directory containing handlabels.csv (file,mylabel)
where mylabel is 0 = junk (not a single readable die) or 1..6 = the die's
top-face value, labeled by visual inspection. These labels are clean by
construction — unlike the roll-derived labels, whose box→turn attribution
was measured at ~25-60% correct (the reason diceboxtrain v1-v5 could not
learn; see the iteration-6/7 notes).

The model doubles as the die DETECTOR (class 0 rejects junk boxes) and the
value reader — the survey's two-stage recipe with the stages folded into
one 7-way head, sized for ~300 labels.

Usage:
    ml/.venv/bin/python train_dielabel.py --crops ../corpus/diceboxes \
        --out out-dielabel [--epochs 120]
"""

import argparse
import csv
import random
from collections import Counter
from pathlib import Path

import numpy as np
import torch
import torch.nn as nn
import torch.nn.functional as F
from PIL import Image

IN = 48


def load_image(path, train):
    img = Image.open(path).convert("RGB")
    if train:
        if random.random() < 0.5:
            img = img.transpose(Image.FLIP_LEFT_RIGHT)
        if random.random() < 0.5:
            img = img.transpose(Image.FLIP_TOP_BOTTOM)
        if random.random() < 0.5:
            img = img.transpose(Image.ROTATE_90)
        w, h = img.size
        dx, dy = random.randint(-4, 4), random.randint(-4, 4)
        img = img.crop((dx, dy, w + dx, h + dy))
    img = img.resize((IN, IN), Image.BILINEAR)
    x = np.asarray(img, dtype=np.float32) / 255.0
    if train:
        x = np.clip(x * random.uniform(0.8, 1.2) + random.uniform(-0.08, 0.08), 0, 1)
    return torch.from_numpy(x.transpose(2, 0, 1))


class DieNet7(nn.Module):
    def __init__(self):
        super().__init__()
        def block(i, o):
            return [nn.Conv2d(i, o, 3, padding=1), nn.BatchNorm2d(o), nn.ReLU(), nn.MaxPool2d(2)]
        self.net = nn.Sequential(
            *block(3, 16), *block(16, 32), *block(32, 64),
            nn.AdaptiveAvgPool2d(1), nn.Flatten(),
            nn.Linear(64, 64), nn.ReLU(), nn.Dropout(0.3),
            nn.Linear(64, 7),
        )

    def forward(self, x):
        return self.net(x)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--crops", required=True)
    ap.add_argument("--out", default="out-dielabel")
    ap.add_argument("--epochs", type=int, default=120)
    ap.add_argument("--val-every", type=int, default=4,
                    help="hold out every Nth recording (by sorted name)")
    ap.add_argument("--seed", type=int, default=7)
    args = ap.parse_args()
    random.seed(args.seed)
    torch.manual_seed(args.seed)

    d = Path(args.crops)
    meta = {r["file"]: r["recording"] for r in csv.DictReader(open(d / "labels.csv"))}
    rows = []
    for r in csv.DictReader(open(d / "handlabels.csv")):
        rows.append({"path": d / r["file"], "rec": meta[r["file"]], "y": int(r["mylabel"])})
    recs = sorted({r["rec"] for r in rows})
    val_recs = {recs[i] for i in range(0, len(recs), args.val_every)}
    train_rows = [r for r in rows if r["rec"] not in val_recs]
    val_rows = [r for r in rows if r["rec"] in val_recs]
    print(f"{len(rows)} hand-labeled crops -> train {len(train_rows)} / val {len(val_rows)}")
    print(f"val recordings: {sorted(val_recs)}")
    print(f"train class counts: {sorted(Counter(r['y'] for r in train_rows).items())}")
    print(f"val   class counts: {sorted(Counter(r['y'] for r in val_rows).items())}", flush=True)

    model = DieNet7()
    opt = torch.optim.Adam(model.parameters(), lr=1e-3)
    sched = torch.optim.lr_scheduler.CosineAnnealingLR(opt, T_max=args.epochs)

    def run_eval(rs):
        model.eval()
        ok = okd = nd = 0
        with torch.no_grad():
            x = torch.stack([load_image(r["path"], False) for r in rs])
            p = model(x).argmax(1)
            for i, r in enumerate(rs):
                ok += int(p[i].item() == r["y"])
                if r["y"] > 0:
                    nd += 1
                    okd += int(p[i].item() == r["y"])
        return ok / max(len(rs), 1), okd / max(nd, 1)

    best, best_state = 0.0, None
    for ep in range(1, args.epochs + 1):
        model.train()
        order = list(range(len(train_rows)))
        random.shuffle(order)
        tot = 0.0
        for i in range(0, len(order), 32):
            chunk = [train_rows[j] for j in order[i:i + 32]]
            x = torch.stack([load_image(r["path"], True) for r in chunk])
            y = torch.tensor([r["y"] for r in chunk])
            opt.zero_grad()
            loss = F.cross_entropy(model(x), y)
            loss.backward()
            opt.step()
            tot += loss.item() * len(chunk)
        sched.step()
        acc, dacc = run_eval(val_rows)
        tacc, _ = run_eval(train_rows)
        star = ""
        if acc > best:
            best, best_state = acc, {k: v.clone() for k, v in model.state_dict().items()}
            star = " *"
        if ep % 4 == 0 or star:
            print(f"epoch {ep:3d}: loss {tot/len(train_rows):.4f}  train {tacc:.3f}  val {acc:.3f} (die-val {dacc:.3f}){star}", flush=True)

    model.load_state_dict(best_state)
    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=True)
    torch.save(model.state_dict(), out / "dielabel.pt")
    acc, dacc = run_eval(val_rows)
    # confusion over val
    conf = np.zeros((7, 7), dtype=int)
    model.eval()
    with torch.no_grad():
        x = torch.stack([load_image(r["path"], False) for r in val_rows])
        p = model(x).argmax(1)
        for i, r in enumerate(val_rows):
            conf[r["y"]][p[i].item()] += 1
    print(f"best val accuracy: {best:.3f} (die-value-only {dacc:.3f})")
    print("val confusion (rows=truth 0..6):")
    for i in range(7):
        print(" ", " ".join(f"{v:3d}" for v in conf[i]))


if __name__ == "__main__":
    main()
