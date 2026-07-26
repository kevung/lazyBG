package dienet

import (
	"image"
	"sort"
)

// Det is one die detection from a band scan.
type Det struct {
	Box   image.Rectangle
	Val   int // 1..6
	Conf  float64
	Probs [NClasses]float64 // full softmax (0 = junk) — feeds the soft pair PMF
}

// ScanBand runs the classifier as a sliding-window die DETECTOR over a
// full-resolution band of a stable frame (detection-by-classification; the
// survey's two-stage recipe with one shared model). The dice of a turn sit
// on the felt in every stable frame of that turn, so scanning the event's
// own frame needs no appearance timing, no phase attribution and no
// temporal stack — the motion-blob proposal funnel this replaces starved
// the cue (measured pilot: 264 appearances → 6 usable values).
//
// sizes are candidate die edge lengths in source pixels; stride is the scan
// step. Windows classified junk (class 0) or below minConf are dropped;
// overlapping survivors are reduced by NMS; at most maxDets detections are
// returned, sorted by confidence.
func ScanBand(net *Net, img *image.RGBA, band image.Rectangle, sizes []int, stride int, minConf float64, maxDets int) []Det {
	band = band.Intersect(img.Bounds())
	var dets []Det
	for _, sz := range sizes {
		pad := sz / 3 // context margin, matching the extractor's crop margin
		for y := band.Min.Y - pad; y+sz+pad <= band.Max.Y+pad+sz/2; y += stride {
			for x := band.Min.X - pad; x+sz+pad <= band.Max.X+pad; x += stride {
				r := image.Rect(x-pad, y-pad, x+sz+pad, y+sz+pad).Intersect(img.Bounds())
				if r.Dx() < 12 || r.Dy() < 12 {
					continue
				}
				probs := ClassifyProbs(net, img.SubImage(r).(*image.RGBA))
				val, conf := 0, 0.0
				for i, v := range probs {
					if v > conf {
						val, conf = i, v
					}
				}
				if val == 0 || conf < minConf {
					continue
				}
				dets = append(dets, Det{Box: r, Val: val, Conf: conf, Probs: probs})
			}
		}
	}
	dets = nms(dets, 0.3)
	if len(dets) > maxDets {
		dets = dets[:maxDets]
	}
	return dets
}

// nms sorts by confidence and greedily suppresses detections whose
// intersection-over-union with an already-kept detection exceeds thresh.
func nms(dets []Det, thresh float64) []Det {
	sort.Slice(dets, func(i, j int) bool { return dets[i].Conf > dets[j].Conf })
	var kept []Det
	for _, d := range dets {
		ok := true
		for _, k := range kept {
			if iou(d.Box, k.Box) > thresh {
				ok = false
				break
			}
		}
		if ok {
			kept = append(kept, d)
		}
	}
	return kept
}

func iou(a, b image.Rectangle) float64 {
	inter := a.Intersect(b)
	if inter.Empty() {
		return 0
	}
	ia := inter.Dx() * inter.Dy()
	ua := a.Dx()*a.Dy() + b.Dx()*b.Dy() - ia
	return float64(ia) / float64(ua)
}
