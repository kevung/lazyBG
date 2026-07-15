#!/usr/bin/env python3
"""Train the board-corner regressor (dev-time only).

Input: crop directories produced by `lazybg cornercrops -manifest ... -out DIR`
(labels.csv + 320x180 PNG frames). Each frame carries the manifest's validated
corner coordinates (TL,TR,BR,BL) in source resolution; targets are normalized
to [0,1] by the source width/height recorded per row.

The task is direct 8-value regression (survey: learned board localization —
a corner heatmap/regressor seeded by the calibration campaign's labels). This
first brick is deliberately small: ~21 camera setups exist in the corpus, so
validation is BY RECORDING and the honest question is "does it land near the
board on an unseen setup", not sub-pixel accuracy — the classical refiner
(opening-oracle / autocal) takes over from a good initialization.

Outputs (to --out):
    cornernet.pt      torch state_dict
    cornernet.json    metadata: input size, normalization, val recordings
    report.txt        per-recording mean corner error (fraction of image diag)

Usage:
    uv run python ml/train_corner.py --crops ../corpus/corners --out out-corner \
        [--val-recordings id1,id2] [--epochs 60]
"""

import argparse
import csv
import json
import os
import random
import sys

import numpy as np
import torch
import torch.nn as nn
import torch.nn.functional as F
from PIL import Image
from torch.utils.data import DataLoader, Dataset

IN_W, IN_H = 160, 90  # halve the stored 320x180: corners are a coarse task


def load_rows(crop_dirs):
    rows = []
    for d in crop_dirs:
        with open(os.path.join(d, "labels.csv")) as f:
            for r in csv.DictReader(f):
                r["_dir"] = d
                rows.append(r)
    return rows


class CornerSet(Dataset):
    def __init__(self, rows, train):
        self.rows = rows
        self.train = train

    def __len__(self):
        return len(self.rows)

    def __getitem__(self, i):
        r = self.rows[i]
        img = Image.open(os.path.join(r["_dir"], r["file"])).convert("RGB")
        w, h = float(r["w"]), float(r["h"])
        pts = np.array(
            [
                [float(r["tlx"]) / w, float(r["tly"]) / h],
                [float(r["trx"]) / w, float(r["try"]) / h],
                [float(r["brx"]) / w, float(r["bry"]) / h],
                [float(r["blx"]) / w, float(r["bly"]) / h],
            ],
            dtype=np.float32,
        )
        if self.train:
            # photometric jitter only — geometry must stay label-true
            arr = np.asarray(img, dtype=np.float32) / 255.0
            arr = arr * random.uniform(0.7, 1.3) + random.uniform(-0.08, 0.08)
            arr = np.clip(arr, 0, 1)
        else:
            arr = np.asarray(img, dtype=np.float32) / 255.0
        t = torch.from_numpy(arr).permute(2, 0, 1)
        t = F.interpolate(t.unsqueeze(0), size=(IN_H, IN_W), mode="bilinear", align_corners=False).squeeze(0)
        return t, torch.from_numpy(pts.reshape(-1))


class CornerNet(nn.Module):
    def __init__(self):
        super().__init__()
        ch = [3, 16, 32, 64, 96]
        layers = []
        for a, b in zip(ch, ch[1:]):
            layers += [nn.Conv2d(a, b, 3, stride=2, padding=1), nn.BatchNorm2d(b), nn.ReLU()]
        self.features = nn.Sequential(*layers)  # 160x90 -> 10x6
        self.head = nn.Sequential(
            nn.AdaptiveAvgPool2d((3, 5)), nn.Flatten(), nn.Linear(96 * 15, 128), nn.ReLU(), nn.Linear(128, 8)
        )

    def forward(self, x):
        return self.head(self.features(x))


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--crops", nargs="+", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--epochs", type=int, default=60)
    ap.add_argument("--val-recordings", default="")
    args = ap.parse_args()

    torch.manual_seed(7)
    random.seed(7)
    rows = load_rows(args.crops)
    recs = sorted({r["recording"] for r in rows})
    if args.val_recordings:
        val_ids = set(args.val_recordings.split(","))
    else:
        val_ids = {recs[i] for i in range(0, len(recs), 5)}  # every 5th setup held out
    train_rows = [r for r in rows if r["recording"] not in val_ids]
    val_rows = [r for r in rows if r["recording"] in val_ids]
    print(f"{len(rows)} frames, {len(recs)} recordings -> train {len(train_rows)} / val {len(val_rows)} "
          f"(val: {sorted(val_ids)})", flush=True)
    if not train_rows or not val_rows:
        sys.exit("empty split")

    tl = DataLoader(CornerSet(train_rows, True), batch_size=32, shuffle=True, num_workers=2)
    vl = DataLoader(CornerSet(val_rows, False), batch_size=32, num_workers=2)
    net = CornerNet()
    opt = torch.optim.Adam(net.parameters(), lr=1e-3)
    sched = torch.optim.lr_scheduler.CosineAnnealingLR(opt, T_max=args.epochs)

    os.makedirs(args.out, exist_ok=True)
    best = None
    for ep in range(1, args.epochs + 1):
        net.train()
        tot = 0.0
        for x, y in tl:
            opt.zero_grad()
            loss = F.smooth_l1_loss(net(x), y)
            loss.backward()
            opt.step()
            tot += loss.item() * len(x)
        sched.step()
        net.eval()
        errs = []
        with torch.no_grad():
            for x, y in vl:
                p = net(x)
                d = (p - y).view(-1, 4, 2)
                errs.append(torch.sqrt((d**2).sum(-1)).mean(1))  # mean corner dist, normalized
        err = torch.cat(errs).mean().item()
        star = ""
        if best is None or err < best:
            best = err
            torch.save(net.state_dict(), os.path.join(args.out, "cornernet.pt"))
            star = " *"
        print(f"epoch {ep:2d}: loss {tot/len(train_rows):.4f}  val corner-err {err:.4f}{star}", flush=True)

    # per-recording report with the best checkpoint
    net.load_state_dict(torch.load(os.path.join(args.out, "cornernet.pt"), weights_only=True))
    net.eval()
    per = {}
    with torch.no_grad():
        for r in val_rows:
            x, y = CornerSet([r], False)[0]
            d = (net(x.unsqueeze(0))[0] - y).view(4, 2)
            per.setdefault(r["recording"], []).append(torch.sqrt((d**2).sum(-1)).mean().item())
    with open(os.path.join(args.out, "report.txt"), "w") as f:
        f.write(f"best mean normalized corner error: {best:.4f}\n")
        for k in sorted(per):
            f.write(f"{k}: {np.mean(per[k]):.4f} over {len(per[k])} frames\n")
    json.dump(
        {"input": [IN_W, IN_H], "targets": "TL,TR,BR,BL xy normalized by source w/h",
         "val_recordings": sorted(val_ids), "best_val_err": best},
        open(os.path.join(args.out, "cornernet.json"), "w"), indent=1)
    print(f"best normalized corner error: {best:.4f}", flush=True)
    print(open(os.path.join(args.out, "report.txt")).read(), flush=True)


if __name__ == "__main__":
    main()
