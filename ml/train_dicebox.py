#!/usr/bin/env python3
"""Train the per-die value classifier on diceevent box crops (two-stage v2).

Input: corpus/diceboxes/ from `lazybg diceboxcrops` — one crop per proposed
die box near an aligned turn, labeled with the turn's roll. Doubles give
unambiguous per-die labels; non-double turns with two boxes share a pairkey
and train with a permutation-invariant pair loss:
    loss = min(CE(a,d1)+CE(b,d2), CE(a,d2)+CE(b,d1))
Split BY RECORDING.

Usage:
    .venv/bin/python train_dicebox.py --crops ../corpus/diceboxes --out out-dicebox
"""

import argparse
import csv
import random
from collections import Counter, defaultdict
from pathlib import Path

import numpy as np
import torch
import torch.nn as nn
import torch.nn.functional as F
from PIL import Image

IN = 48  # square die crop


def load_rows(d):
    d = Path(d)
    rows = []
    with open(d / "labels.csv") as f:
        for r in csv.DictReader(f):
            rows.append({
                "path": d / r["file"], "rec": r["recording"],
                "d1": int(r["d1"]), "d2": int(r["d2"]),
                "double": r["double"] == "1", "pair": r["pairkey"],
            })
    return rows


def load_image(row, train):
    img = Image.open(row["path"]).convert("RGB")
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
        x = np.clip(x * random.uniform(0.85, 1.15) + random.uniform(-0.06, 0.06), 0, 1)
    return torch.from_numpy(x.transpose(2, 0, 1))


class DieNet(nn.Module):
    def __init__(self):
        super().__init__()
        def block(i, o):
            return [nn.Conv2d(i, o, 3, padding=1), nn.BatchNorm2d(o), nn.ReLU(), nn.MaxPool2d(2)]
        self.net = nn.Sequential(
            *block(3, 16), *block(16, 32), *block(32, 64),
            nn.AdaptiveAvgPool2d(1), nn.Flatten(),
            nn.Linear(64, 64), nn.ReLU(), nn.Dropout(0.2),
            nn.Linear(64, 6),
        )

    def forward(self, x):
        return self.net(x)


def batches(units, bs, train):
    order = list(range(len(units)))
    if train:
        random.shuffle(order)
    for i in range(0, len(order), bs):
        yield [units[j] for j in order[i:i + bs]]


def unit_loss(model, unit, train):
    # unit: {"kind":"single","row":r} (doubles: label known)
    #       {"kind":"pair","rows":[a,b]} (permutation-invariant)
    if unit["kind"] == "single":
        x = load_image(unit["row"], train).unsqueeze(0)
        y = torch.tensor([unit["row"]["d1"] - 1])
        return F.cross_entropy(model(x), y)
    a, b = unit["rows"]
    xa = load_image(a, train).unsqueeze(0)
    xb = load_image(b, train).unsqueeze(0)
    la, lb = model(xa), model(xb)
    d1, d2 = a["d1"] - 1, a["d2"] - 1
    l1 = F.cross_entropy(la, torch.tensor([d1])) + F.cross_entropy(lb, torch.tensor([d2]))
    l2 = F.cross_entropy(la, torch.tensor([d2])) + F.cross_entropy(lb, torch.tensor([d1]))
    return torch.minimum(l1, l2) / 2


def evaluate(model, units):
    model.eval()
    ok, n = 0, 0
    with torch.no_grad():
        for u in units:
            if u["kind"] == "single":
                p = model(load_image(u["row"], False).unsqueeze(0)).argmax(1).item()
                ok += int(p == u["row"]["d1"] - 1)
                n += 1
            else:
                a, b = u["rows"]
                pa = model(load_image(a, False).unsqueeze(0)).argmax(1).item()
                pb = model(load_image(b, False).unsqueeze(0)).argmax(1).item()
                want = sorted([a["d1"] - 1, a["d2"] - 1])
                ok += int(sorted([pa, pb]) == want) * 2
                n += 2
    return ok / max(n, 1)


def to_units(rows):
    bypair = defaultdict(list)
    for r in rows:
        bypair[r["pair"]].append(r)
    units = []
    for _, rs in bypair.items():
        if rs[0]["double"]:
            for r in rs:
                units.append({"kind": "single", "row": r})
        elif len(rs) == 2:
            units.append({"kind": "pair", "rows": rs})
        # 1 or 3 boxes on a non-double: ambiguous, skipped
    return units


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--crops", required=True)
    ap.add_argument("--out", default="out-dicebox")
    ap.add_argument("--epochs", type=int, default=40)
    ap.add_argument("--val-recordings", default="",
                    help="comma-separated; default holds out every 4th recording — the v2/v3 single-recording val (~25 units) measured noise, not skill")
    ap.add_argument("--min-contrast", type=int, default=45,
                    help="drop crops whose luminance p95-p5 is below this (junk boxes: uniform felt); 0 disables")
    ap.add_argument("--seed", type=int, default=7)
    args = ap.parse_args()
    random.seed(args.seed)
    torch.manual_seed(args.seed)

    rows = load_rows(args.crops)
    if args.min_contrast > 0:
        kept = []
        for r in rows:
            g = np.asarray(Image.open(r["path"]).convert("L"), dtype=np.float32)
            if np.percentile(g, 95) - np.percentile(g, 5) >= args.min_contrast:
                kept.append(r)
        print(f"contrast filter: kept {len(kept)}/{len(rows)} crops (min p95-p5 {args.min_contrast})")
        rows = kept
    if args.val_recordings:
        val_recs = set(args.val_recordings.split(","))
    else:
        recs = sorted({r["rec"] for r in rows})
        val_recs = {recs[i] for i in range(0, len(recs), 4)}
        print(f"val recordings: {sorted(val_recs)}")
    train_units = to_units([r for r in rows if r["rec"] not in val_recs])
    val_units = to_units([r for r in rows if r["rec"] in val_recs])
    n_single = sum(1 for u in train_units if u["kind"] == "single")
    print(f"{len(rows)} crops -> train units {len(train_units)} ({n_single} unambiguous) / val units {len(val_units)}")

    model = DieNet()
    opt = torch.optim.Adam(model.parameters(), lr=1e-3)
    sched = torch.optim.lr_scheduler.CosineAnnealingLR(opt, T_max=args.epochs)

    best, best_state = 0.0, None
    for ep in range(args.epochs):
        model.train()
        tot, n = 0.0, 0
        for chunk in batches(train_units, 32, True):
            opt.zero_grad()
            loss = torch.stack([unit_loss(model, u, True) for u in chunk]).mean()
            loss.backward()
            opt.step()
            tot += loss.item() * len(chunk)
            n += len(chunk)
        sched.step()
        acc = evaluate(model, val_units)
        star = ""
        if acc > best:
            best, best_state = acc, {k: v.clone() for k, v in model.state_dict().items()}
            star = " *"
        print(f"epoch {ep+1:2d}: loss {tot/max(n,1):.4f}  val per-die {acc:.3f}{star}")

    model.load_state_dict(best_state)
    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=True)
    torch.save(model.state_dict(), out / "dicebox.pt")
    print(f"best per-die accuracy: {best:.3f} (chance 0.167)")


if __name__ == "__main__":
    main()
