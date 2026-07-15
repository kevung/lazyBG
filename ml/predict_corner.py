#!/usr/bin/env python3
"""Predict board corners on a raw video (dev-time autocal seeding).

Grabs a frame with ffmpeg, runs the corner regressor, prints the corner
coordinates in source resolution as the exact string `lazybg autocal
-init-corners` expects — the bridge from "video nobody has calibrated"
to the opening-oracle refiner:

    ml/.venv/bin/python ml/predict_corner.py --video V.mkv [--at-ms 60000]
    lazybg autocal -video V.mkv ... -init-corners $(...)

Prints one line: TLx,TLy,TRx,TRy,BRx,BRy,BLx,BLy (integers).
"""

import argparse
import io
import subprocess
import sys

import numpy as np
import torch
import torch.nn.functional as F
from PIL import Image

from train_corner import IN_H, IN_W, CornerNet


def grab_frame(video, at_ms):
    out = subprocess.run(
        ["ffmpeg", "-v", "quiet", "-ss", str(at_ms / 1000), "-i", video,
         "-frames:v", "1", "-f", "image2", "-c:v", "png", "-"],
        capture_output=True, check=True).stdout
    return Image.open(io.BytesIO(out)).convert("RGB")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--video", required=True)
    ap.add_argument("--model", default="out-corner/cornernet.pt")
    ap.add_argument("--at-ms", type=int, default=60000)
    args = ap.parse_args()

    img = grab_frame(args.video, args.at_ms)
    W, H = img.size
    x = torch.from_numpy(np.asarray(img, dtype=np.float32) / 255.0).permute(2, 0, 1)
    x = F.interpolate(x.unsqueeze(0), size=(IN_H, IN_W), mode="bilinear", align_corners=False)

    net = CornerNet()
    net.load_state_dict(torch.load(args.model, weights_only=True))
    net.eval()
    with torch.no_grad():
        p = net(x)[0].view(4, 2).numpy()
    coords = []
    for i in range(4):
        coords += [str(round(float(p[i][0]) * W)), str(round(float(p[i][1]) * H))]
    print(",".join(coords))
    return 0


if __name__ == "__main__":
    sys.exit(main())
