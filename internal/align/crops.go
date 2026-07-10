package align

import (
	"encoding/csv"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"

	"lazybg/internal/bg"
	"lazybg/internal/capture"
	"lazybg/internal/corpus"
	"lazybg/internal/perceive/boarddiff"
	"lazybg/internal/transcribe"
)

// CropsResult summarizes an extraction.
type CropsResult struct {
	Turns int // aligned turns a frame was decoded for
	Crops int // labeled point crops written
}

// ExtractCrops writes one labeled crop per board point for every aligned
// turn: the rectified point region as PNG plus a labels.csv row carrying the
// truth (count, owner) — the training corpus for the learned board reader
// (experiment-plan §5: board labels are free once the tick is known).
// Turns whose assigned event agrees with the truth board below minScore are
// skipped: a misaligned frame would poison the labels. Only single-Part
// recordings are supported for now.
func ExtractCrops(root string, m corpus.Manifest, turns []Turn, assign []int, events []transcribe.Event, outDir string, minScore float64) (CropsResult, error) {
	if len(m.Parts) != 1 {
		return CropsResult{}, fmt.Errorf("crops: only single-part recordings supported, got %d parts", len(m.Parts))
	}
	part := m.Parts[0]
	cal, cb, _, err := transcribe.PartSetup(part)
	if err != nil {
		return CropsResult{}, err
	}
	video := filepath.Join(root, part.File)

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return CropsResult{}, err
	}
	lf, err := os.Create(filepath.Join(outDir, "labels.csv"))
	if err != nil {
		return CropsResult{}, err
	}
	defer lf.Close()
	w := csv.NewWriter(lf)
	defer w.Flush()
	if err := w.Write([]string{"file", "recording", "game", "index", "tick_ms", "point", "count", "owner"}); err != nil {
		return CropsResult{}, err
	}

	var res CropsResult
	for k, turn := range turns {
		if assign[k] < 0 {
			continue
		}
		if boarddiff.WholeBoardMatch(turn.Board, events[assign[k]].Obs) < minScore {
			continue
		}
		tick := events[assign[k]].Tick
		frame, err := capture.FrameAt(video, tick)
		if err != nil {
			continue
		}
		rect := cal.Rectify(frame)
		res.Turns++
		for p := 1; p <= 24; p++ {
			region, _ := cb.PointRegion(p)
			crop := cropImage(rect, region)
			name := fmt.Sprintf("g%d_i%d_p%d.png", turn.Game, turn.Index, p)
			f, err := os.Create(filepath.Join(outDir, name))
			if err != nil {
				return res, err
			}
			if err := png.Encode(f, crop); err != nil {
				f.Close()
				return res, err
			}
			f.Close()

			count, owner := labelOf(turn.Board, p)
			if err := w.Write([]string{
				name, m.ID,
				strconv.Itoa(turn.Game), strconv.Itoa(turn.Index), strconv.Itoa(tick),
				strconv.Itoa(p), strconv.Itoa(count), owner,
			}); err != nil {
				return res, err
			}
			res.Crops++
		}
	}
	return res, w.Error()
}

// labelOf renders a truth point as (count, owner) with owner "A" (P1), "B"
// (P2) or "-" (empty) — the manifest's CheckerA is P1 by convention.
func labelOf(b bg.Board, p int) (int, string) {
	pt := b.Pts[p]
	if pt.N == 0 {
		return 0, "-"
	}
	if pt.Owner == bg.P2 {
		return pt.N, "B"
	}
	return pt.N, "A"
}

// cropImage copies a region of a rectified board image.
func cropImage(img *image.RGBA, r image.Rectangle) *image.RGBA {
	r = r.Intersect(img.Bounds())
	out := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			out.Set(x, y, img.At(r.Min.X+x, r.Min.Y+y))
		}
	}
	return out
}
