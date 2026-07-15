#!/usr/bin/env python3
"""Train the learned dice-value reader on weakly-supervised band crops.

Input: corpus/dicecrops/ produced by `lazybg dicecrops` — the rectified
central felt band at every truth-aligned turn tick, labeled with the roll
(d1<=d2) from the .mat. No localization labels: the network must find the
dice inside the band itself (the classical pip reader manages 5% exact
pairs on this data — docs/research/dice-reading-survey.md).

Task: 21-way classification (unordered pairs 11,12,...,66).
Split: BY RECORDING (never by crop).

Usage:
    .venv/bin/python train_dicereader.py --crops ../corpus/dicecrops \
        --out out-dice [--val-recordings id1,id2]
"""

import argparse
import csv
import random
from collections import Counter
from pathlib import Path

import numpy as np
import torch
import torch.nn as nn
from PIL import Image

IN_W, IN_H = 352, 80  # band is ~780x174 at canonical scale; keep the aspect

PAIRS = [(a, b) for a in range(1, 7) for b in range(a, 7)]
PAIR_IDX = {p: i for i, p in enumerate(PAIRS)}


def load_rows(d):
    d = Path(d)
    rows = []
    with open(d / "labels.csv") as f:
        for r in csv.DictReader(f):
            rows.append({
                "path": d / r["file"],
                "rec": r["recording"],
                "cls": PAIR_IDX[(int(r["d1"]), int(r["d2"]))],
            })
    return rows


def load_image(row, train):
    img = Image.open(row["path"]).convert("RGB")
    if train:
        if random.random() < 0.5:
            img = img.transpose(Image.FLIP_LEFT_RIGHT)
        if random.random() < 0.5:
            img = img.transpose(Image.FLIP_TOP_BOTTOM)
        w, h = img.size
        dx, dy = random.randint(-8, 8), random.randint(-4, 4)
        img = img.crop((dx, dy, w + dx, h + dy))
    img = img.resize((IN_W, IN_H), Image.BILINEAR)
    x = np.asarray(img, dtype=np.float32) / 255.0
    if train:
        x = np.clip(x * random.uniform(0.85, 1.15) + random.uniform(-0.06, 0.06), 0, 1)
    return torch.from_numpy(x.transpose(2, 0, 1))


class Batches:
    def __init__(self, rows, batch, train):
        self.rows, self.batch, self.train = rows, batch, train

    def __iter__(self):
        order = list(range(len(self.rows)))
        if self.train:
            random.shuffle(order)
        for i in range(0, len(order), self.batch):
            chunk = [self.rows[j] for j in order[i:i + self.batch]]
            x = torch.stack([load_image(r, self.train) for r in chunk])
            y = torch.tensor([r["cls"] for r in chunk], dtype=torch.long)
            yield x, y


class DiceNet(nn.Module):
    def __init__(self):
        super().__init__()
        def block(i, o):
            return [nn.Conv2d(i, o, 3, padding=1), nn.BatchNorm2d(o), nn.ReLU(), nn.MaxPool2d(2)]
        self.net = nn.Sequential(
            *block(3, 24), *block(24, 48), *block(48, 96), *block(96, 96),
            nn.AdaptiveAvgPool2d(1), nn.Flatten(),
            nn.Linear(96, 96), nn.ReLU(), nn.Dropout(0.3),
            nn.Linear(96, len(PAIRS)),
        )

    def forward(self, x):
        return self.net(x)


def evaluate(model, rows):
    model.eval()
    exact, onedie, n = 0, 0, 0
    with torch.no_grad():
        for x, y in Batches(rows, 96, train=False):
            pred = model(x).argmax(1)
            for t, p in zip(y.tolist(), pred.tolist()):
                n += 1
                if t == p:
                    exact += 1
                elif set(PAIRS[t]) & set(PAIRS[p]):
                    onedie += 1
    return exact / max(n, 1), onedie / max(n, 1)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--crops", required=True)
    ap.add_argument("--out", default="out-dice")
    ap.add_argument("--epochs", type=int, default=50)
    ap.add_argument("--val-recordings", default="2025-05_hsbtMarseille_or-r1_HanotinDenis")
    ap.add_argument("--seed", type=int, default=7)
    args = ap.parse_args()
    random.seed(args.seed)
    torch.manual_seed(args.seed)

    rows = load_rows(args.crops)
    val_recs = set(args.val_recordings.split(","))
    train = [r for r in rows if r["rec"] not in val_recs]
    val = [r for r in rows if r["rec"] in val_recs]
    print(f"{len(rows)} bands -> train {len(train)} / val {len(val)} (val: {sorted(val_recs)})")
    print("pair distribution (train):", dict(sorted(Counter(r['cls'] for r in train).items())))

    model = DiceNet()
    opt = torch.optim.Adam(model.parameters(), lr=1e-3)
    sched = torch.optim.lr_scheduler.CosineAnnealingLR(opt, T_max=args.epochs)
    lossf = nn.CrossEntropyLoss()

    best, best_state = 0.0, None
    for ep in range(args.epochs):
        model.train()
        tot, n = 0.0, 0
        for x, y in Batches(train, 64, train=True):
            opt.zero_grad()
            loss = lossf(model(x), y)
            loss.backward()
            opt.step()
            tot += loss.item() * len(y)
            n += len(y)
        sched.step()
        exact, onedie = evaluate(model, val)
        star = ""
        if exact > best:
            best, best_state = exact, {k: v.clone() for k, v in model.state_dict().items()}
            star = " *"
        print(f"epoch {ep+1:2d}: loss {tot/max(n,1):.4f}  val exact {exact:.3f} one-die {onedie:.3f}{star}")

    model.load_state_dict(best_state)
    exact, onedie = evaluate(model, val)
    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=True)
    torch.save(model.state_dict(), out / "dicereader.pt")
    print(f"best: exact-pair {exact:.3f}, one-die {onedie:.3f} (classical pip reader: 0.05 exact)")
    print(f"saved {out}/dicereader.pt")


if __name__ == "__main__":
    main()
