// Command lazybg transcribes backgammon match videos.
//
//	lazybg transcribe -manifest corpus/manifest/X.json [-out X.mat] [-limit-ms N]
//	    run the video pipeline over a Recording manifest and write the .mat
//	lazybg eval -manifest corpus/manifest/X.json [-limit-ms N]
//	    transcribe, then score against the manifest's ground-truth transcript
//	lazybg align -manifest corpus/manifest/X.json [-write-manifest] [-crops DIR] [-limit-ms N]
//	    anchor the ground-truth transcript to the video (per-turn ticks) and
//	    optionally write the aligned manifest and labeled training crops
//	lazybg demo
//	    the original engine-spine demo on synthetic observations
//
// The review UI (video scrubber ↔ move list ↔ review queue) arrives at the UI
// milestone (docs/architecture.md §3, §8).
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"

	"encoding/json"

	"lazybg"
	"lazybg/internal/align"
	"lazybg/internal/autocal"
	"lazybg/internal/bg"
	"lazybg/internal/capture"
	"lazybg/internal/corpus"
	"lazybg/internal/cue"
	"lazybg/internal/derive"
	"lazybg/internal/engine"
	"lazybg/internal/eval"
	"lazybg/internal/fusion"
	"lazybg/internal/gate"
	"lazybg/internal/geom"
	"lazybg/internal/matexport"
	"lazybg/internal/matimport"
	"lazybg/internal/perceive"
	"lazybg/internal/perceive/boarddiff"
	"lazybg/internal/perceive/diceevent"
	"lazybg/internal/perceive/dienet"
	"lazybg/internal/profile"
	"lazybg/internal/transcribe"
)

func main() {
	if err := engine.Init(lazybg.DataFS); err != nil {
		log.Fatalf("engine init: %v", err)
	}
	cmd := "demo"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	switch cmd {
	case "demo":
		runDemo()
	case "transcribe":
		runTranscribe(os.Args[2:])
	case "eval":
		runEval(os.Args[2:])
	case "align":
		runAlign(os.Args[2:])
	case "autocal":
		runAutocal(os.Args[2:])
	case "dicecrops":
		runDicecrops(os.Args[2:])
	case "diceboxcrops":
		runDiceboxcrops(os.Args[2:])
	case "cornercrops":
		runCornercrops(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q (want transcribe, eval or demo)\n", cmd)
		os.Exit(2)
	}
}

// loadManifest reads a Recording manifest; paths inside are relative to the
// current directory (run from the repository root).
func loadManifest(path string) corpus.Manifest {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read manifest: %v", err)
	}
	m, err := corpus.Load(data)
	if err != nil {
		log.Fatalf("manifest: %v", err)
	}
	return m
}

func runPipeline(manifest string, limitMs int, model, diceModel string) (transcribe.Outcome, corpus.Manifest) {
	m := loadManifest(manifest)
	o := transcribe.DefaultRunOptions()
	o.LimitMs = limitMs
	o.ModelPath = model
	switch diceModel {
	case "none":
		// dice-value cue disabled
	case "embedded":
		raw, err := lazybg.DataFS.ReadFile("data/models/dievalue.bin")
		if err != nil {
			log.Fatalf("embedded dice model: %v", err)
		}
		net, err := dienet.LoadBytes(raw)
		if err != nil {
			log.Fatalf("embedded dice model: %v", err)
		}
		o.DiceNet = net
	default:
		o.DiceModelPath = diceModel
	}
	o.Log = os.Stderr
	out, err := transcribe.Recording(".", m, o)
	if err != nil {
		log.Fatalf("transcribe: %v", err)
	}
	fmt.Fprintf(os.Stderr, "plies: %d games: %d review: %d skipped: %d unexplained: %d\n",
		countPlies(out.Match), len(out.Match.Games), len(out.Review), out.Skipped, out.Unexplained)
	return out, m
}

func runTranscribe(args []string) {
	fs := flag.NewFlagSet("transcribe", flag.ExitOnError)
	manifest := fs.String("manifest", "", "Recording manifest JSON (required)")
	outPath := fs.String("out", "", ".mat output path (default stdout)")
	limitMs := fs.Int("limit-ms", 0, "stop each part this many ms after its span begins (0 = full span)")
	model := fs.String("model", "", "read boards with this learned point-reader weight file")
	diceModel := fs.String("dice-model", "embedded", "die-value cue weights: \"embedded\" (shipped model), \"none\" (off), or a weight-file path")
	fs.Parse(args)
	if *manifest == "" {
		fs.Usage()
		os.Exit(2)
	}
	out, _ := runPipeline(*manifest, *limitMs, *model, *diceModel)
	mat := matexport.Write(out.Match)
	if *outPath == "" {
		fmt.Print(mat)
		return
	}
	if err := os.WriteFile(*outPath, []byte(mat), 0o644); err != nil {
		log.Fatal(err)
	}
}

func runEval(args []string) {
	fs := flag.NewFlagSet("eval", flag.ExitOnError)
	manifest := fs.String("manifest", "", "Recording manifest JSON (required)")
	limitMs := fs.Int("limit-ms", 0, "stop each part this many ms after its span begins (0 = full span)")
	model := fs.String("model", "", "read boards with this learned point-reader weight file")
	diceModel := fs.String("dice-model", "embedded", "die-value cue weights: \"embedded\" (shipped model), \"none\" (off), or a weight-file path")
	fs.Parse(args)
	if *manifest == "" {
		fs.Usage()
		os.Exit(2)
	}
	out, m := runPipeline(*manifest, *limitMs, *model, *diceModel)
	matBytes, err := os.ReadFile(m.Transcript)
	if err != nil {
		log.Fatalf("truth transcript: %v", err)
	}
	truth, err := matimport.Parse(string(matBytes))
	if err != nil {
		log.Fatalf("truth transcript: %v", err)
	}
	threshold := transcribe.DefaultOptions().Policy.Threshold
	s := eval.ScoreMatch(out.Match, truth, threshold)
	fmt.Printf("gate used:        %.2f\n", threshold)
	fmt.Printf("games:            %d produced / %d truth\n", s.GamesOut, s.GamesTruth)
	fmt.Printf("checker plies:    %d produced / %d truth\n", s.OutCheckerPlies, s.TruthCheckerPlies)
	fmt.Printf("matched:          %d\n", s.Matched)
	fmt.Printf("auto-filled:      %d (errors %d, rate %.3f)\n", s.AutoFilled, s.AutoFillErrors(), s.ErrorRate())
	fmt.Printf("coverage:         %.3f\n", s.Coverage())
	fmt.Printf("review rate:      %.3f\n", s.ReviewRate())
	fmt.Printf("truth cube plays: %d (not yet perceived)\n", s.TruthCubeActions)
	// Threshold calibration table: the same transcription scored at several
	// gates — the empirical basis for picking the auto-fill threshold
	// (locked decision #4: hand-set first, calibrated on labeled data now).
	fmt.Printf("threshold table:  gate autofill errors coverage\n")
	for _, th := range []float64{0.60, 0.65, 0.70, 0.75, 0.80, 0.85} {
		ts := eval.ScoreMatch(out.Match, truth, th)
		fmt.Printf("                  %.2f %8d %6d %8.3f\n", th, ts.AutoFilled, ts.AutoFillErrors(), ts.Coverage())
	}
}

// runAutocal auto-calibrates one video and writes a Recording manifest —
// the scaling step: video + .mat in, committed manifest out; `align` then
// labels it. Colors are derived from the footage unless declared.
func runAutocal(args []string) {
	fs := flag.NewFlagSet("autocal", flag.ExitOnError)
	video := fs.String("video", "", "video path(s), comma-separated for multi-part matches (required; part 1 is calibrated, later parts inherit)")
	transcript := fs.String("transcript", "", "ground-truth .mat path (required)")
	id := fs.String("id", "", "recording id (required)")
	outManifest := fs.String("out-manifest", "", "manifest JSON to write (required)")
	checkerA := fs.String("checkerA", "#e1ded2", "CheckerA (P1) hex color prior")
	checkerB := fs.String("checkerB", "#464850", "CheckerB (P2) hex color prior")
	minOpening := fs.Int("min-opening", 19, "reject calibrations whose opening read (of 24) is below this; the per-turn 0.80 crop filter still guards label quality downstream")
	initCorners := fs.String("init-corners", "", "assisted mode: 8 comma-separated numbers (TLx,TLy,TRx,TRy,BRx,BRy,BLx,BLy) — skip board detection, refine these corners against the opening oracle")
	fs.Parse(args)
	if *video == "" || *transcript == "" || *id == "" || *outManifest == "" {
		fs.Usage()
		os.Exit(2)
	}

	videos := strings.Split(*video, ",")

	matBytes, err := os.ReadFile(*transcript)
	if err != nil {
		log.Fatalf("transcript: %v", err)
	}
	truth, err := matimport.Parse(string(matBytes))
	if err != nil {
		log.Fatalf("transcript: %v", err)
	}

	o := autocal.DefaultOptions()
	o.MinOpening = *minOpening
	ca, err := profile.ParseHex(*checkerA)
	if err != nil {
		log.Fatal(err)
	}
	cb, err := profile.ParseHex(*checkerB)
	if err != nil {
		log.Fatal(err)
	}
	o.Profile = profile.CaptureProfile{CheckerA: ca, CheckerB: cb}

	var res autocal.Result
	if *initCorners != "" {
		var v [8]float64
		if _, e := fmt.Sscanf(*initCorners, "%f,%f,%f,%f,%f,%f,%f,%f",
			&v[0], &v[1], &v[2], &v[3], &v[4], &v[5], &v[6], &v[7]); e != nil {
			log.Fatalf("init-corners: %v", e)
		}
		initial := [4]geom.Pt{geom.P(v[0], v[1]), geom.P(v[2], v[3]), geom.P(v[4], v[5]), geom.P(v[6], v[7])}
		res, err = autocal.CalibrateAssisted(videos[0], initial, o)
	} else {
		res, err = autocal.Calibrate(videos[0], o)
	}
	if err != nil {
		log.Fatalf("autocal %s: %v (got %+v)", *id, err, res)
	}
	durMs, err := capture.DurationMs(videos[0])
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stderr, "%s: opening %d/24 @%dms, corners %v\n", *id, res.OpeningScore, res.SpanBeginMs, res.Corners)

	cornersJSON := make([][2]float64, 4)
	for i, p := range res.Corners {
		cornersJSON[i] = [2]float64{p.X, p.Y}
	}
	m := corpus.Manifest{
		SchemaVersion: corpus.SchemaVersion,
		ID:            *id,
		Transcript:    *transcript,
		Cell:          corpus.Cell{Dice: "opaque", Audio: "table"},
		Parts: []corpus.Part{{
			File: videos[0],
			Priors: corpus.Priors{
				Clock: true, MatchLength: truth.Length,
				CheckerA: *checkerA, CheckerB: *checkerB,
				Orientation: "p1-bottom",
			},
			Calibration: corpus.Calibration{
				Corners: cornersJSON,
				Canonical: &corpus.Canonical{MarginX: 16, MarginY: 18,
					PointW: 58, QuadH: 300, BarGap: 60, OffW: 24},
				OpeningScore: res.OpeningScore,
			},
			Span: corpus.Span{BeginMs: res.SpanBeginMs, EndMs: durMs},
		}},
	}
	// Later parts: same table, same calibration (inherit); play resumes
	// immediately, so the span is the whole file.
	for _, v := range videos[1:] {
		d, err := capture.DurationMs(v)
		if err != nil {
			log.Fatal(err)
		}
		m.Parts = append(m.Parts, corpus.Part{
			File:        v,
			Priors:      corpus.Priors{Inherit: true},
			Calibration: corpus.Calibration{Inherit: true},
			Span:        corpus.Span{BeginMs: 0, EndMs: d},
		})
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*outManifest, append(data, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", *outManifest)
}

// runDicecrops extracts the weakly-supervised dice dataset: at every
// truth-aligned turn tick, the rectified central felt band (where dice land)
// plus the roll that produced the turn (from the .mat) — no localization
// labels needed. Training input for the learned dice-value reader
// (docs/research/dice-reading-survey.md: values are beyond the classical
// pip reader at 720p; the survey's weak-supervision plan).
func runDicecrops(args []string) {
	fs := flag.NewFlagSet("dicecrops", flag.ExitOnError)
	manifest := fs.String("manifest", "", "Recording manifest JSON (required)")
	outDir := fs.String("out", "", "output directory (required)")
	fs.Parse(args)
	if *manifest == "" || *outDir == "" {
		fs.Usage()
		os.Exit(2)
	}
	m := loadManifest(*manifest)
	matBytes, err := os.ReadFile(m.Transcript)
	if err != nil {
		log.Fatalf("transcript: %v", err)
	}
	truth, err := matimport.Parse(string(matBytes))
	if err != nil {
		log.Fatalf("transcript: %v", err)
	}
	states := derive.Replay(truth)
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatal(err)
	}
	lf, err := os.OpenFile(filepath.Join(*outDir, "labels.csv"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatal(err)
	}
	defer lf.Close()
	if st, _ := lf.Stat(); st.Size() == 0 {
		fmt.Fprintln(lf, "file,recording,index,tick_ms,d1,d2")
	}

	n := 0
	for _, tn := range m.Turns {
		if tn.Index-1 >= len(states) {
			continue
		}
		ts := states[tn.Index-1]
		if ts.Dice[0] == 0 {
			continue
		}
		part := m.Parts[tn.Part]
		cal, cb, _, err := transcribe.PartSetup(part)
		if err != nil {
			continue
		}
		frame, err := capture.FrameAt(part.File, tn.TickMs)
		if err != nil {
			continue
		}
		rect := cal.Rectify(frame)
		w, h := cb.Size()
		band := image.Rect(cb.MarginX, cb.MarginY+cb.QuadH-cb.PointW, w-cb.MarginX, h-cb.MarginY-cb.QuadH+cb.PointW)
		crop := rect.SubImage(band)
		d1, d2 := ts.Dice[0], ts.Dice[1]
		if d1 > d2 {
			d1, d2 = d2, d1
		}
		name := fmt.Sprintf("%s_i%d.png", m.ID, tn.Index)
		f, err := os.Create(filepath.Join(*outDir, name))
		if err != nil {
			log.Fatal(err)
		}
		if err := png.Encode(f, crop); err != nil {
			f.Close()
			log.Fatal(err)
		}
		f.Close()
		fmt.Fprintf(lf, "%s,%s,%d,%d,%d,%d\n", name, m.ID, tn.Index, tn.TickMs, d1, d2)
		n++
	}
	fmt.Fprintf(os.Stderr, "%s: %d dice-band crops\n", m.ID, n)
}

// runDiceboxcrops extracts per-die crops: the dice-appearance detector
// (validated at 35/35 turn coverage) proposes boxes on the central felt
// band; each box near an aligned turn is cropped at full resolution and
// labeled with that turn's roll — doubles give unambiguous per-die labels,
// non-doubles carry the pair for a permutation-invariant training loss.
// This is the survey's two-stage recipe (proposal + tiny crop classifier).
func runDiceboxcrops(args []string) {
	fs := flag.NewFlagSet("diceboxcrops", flag.ExitOnError)
	manifest := fs.String("manifest", "", "Recording manifest JSON (required)")
	outDir := fs.String("out", "", "output directory (required)")
	fs.Parse(args)
	if *manifest == "" || *outDir == "" {
		fs.Usage()
		os.Exit(2)
	}
	m := loadManifest(*manifest)
	matBytes, err := os.ReadFile(m.Transcript)
	if err != nil {
		log.Fatalf("transcript: %v", err)
	}
	truth, err := matimport.Parse(string(matBytes))
	if err != nil {
		log.Fatalf("transcript: %v", err)
	}
	states := derive.Replay(truth)
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatal(err)
	}
	lf, err := os.OpenFile(filepath.Join(*outDir, "labels.csv"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatal(err)
	}
	defer lf.Close()
	if st, _ := lf.Stat(); st.Size() == 0 {
		fmt.Fprintln(lf, "file,recording,index,tick_ms,d1,d2,double,pairkey")
	}

	const sw, sh = 320, 180
	total := 0
	for pi, part := range m.Parts {
		probe, err := capture.FrameAt(part.File, part.Span.BeginMs)
		if err != nil {
			continue
		}
		srcW := probe.Bounds().Dx()
		srcH := probe.Bounds().Dy()
		minX, minY := part.Calibration.Corners[0][0], part.Calibration.Corners[0][1]
		maxX, maxY := minX, minY
		for _, c := range part.Calibration.Corners[1:] {
			minX, maxX = min(minX, c[0]), max(maxX, c[0])
			minY, maxY = min(minY, c[1]), max(maxY, c[1])
		}
		sx, sy := float64(sw)/float64(srcW), float64(sh)/float64(srcH)
		bandTop := minY + 0.40*(maxY-minY)
		bandBot := minY + 0.60*(maxY-minY)

		src, err := capture.Stream(part.File, capture.StreamOpts{
			BeginMs: part.Span.BeginMs, EndMs: part.Span.EndMs, FPS: 3, W: sw, H: sh})
		if err != nil {
			continue
		}
		dd := diceevent.New(diceevent.Options{})
		type appear struct {
			tick int
			box  image.Rectangle
		}
		var appears []appear
		for {
			f, ok := src.Next()
			if !ok {
				break
			}
			sub := f.Img.(*image.RGBA).SubImage(image.Rect(
				int(minX*sx), int(bandTop*sy), int(maxX*sx), int(bandBot*sy))).(*image.RGBA)
			for _, ev := range dd.Feed(capture.Frame{Tick: f.Tick, Img: sub}) {
				if ev.Kind == diceevent.Appeared {
					appears = append(appears, appear{ev.Tick, ev.Box})
				}
			}
		}
		src.Close()

		// Attribute appearances to turns by ROLL PHASE (world-model v1):
		// a turn's dice appear between the previous commit and its own —
		// cue.AttributeRoll replaces the fixed [-15s,+4s] window the label
		// audit measured at ~25-60% correct.
		var partTurns []corpus.Turn
		var partTicks []int
		for _, tn := range m.Turns {
			if tn.Part == pi && tn.Index-1 < len(states) {
				partTurns = append(partTurns, tn)
				partTicks = append(partTicks, tn.TickMs)
			}
		}
		byTurn := map[int][]appear{}
		for _, a := range appears {
			// Die-shaped only: at stream scale dice read ~4-10px per
			// side and near-square; checkers are larger, point-tip
			// slivers are elongated.
			dx, dy := a.box.Dx(), a.box.Dy()
			if dx < 3 || dy < 3 || dx > 10 || dy > 10 {
				continue
			}
			if dx*2 < dy || dy*2 < dx {
				continue
			}
			k, ok := cue.AttributeRoll(partTicks, a.tick, 3000, 2000)
			if !ok {
				continue
			}
			byTurn[k] = append(byTurn[k], a)
		}
		for k, tn := range partTurns {
			ts := states[tn.Index-1]
			if ts.Dice[0] == 0 {
				continue
			}
			d1, d2 := ts.Dice[0], ts.Dice[1]
			if d1 > d2 {
				d1, d2 = d2, d1
			}
			near := byTurn[k]
			if len(near) == 0 || len(near) > 2 {
				continue // strict: exactly the two dice (or one visible)
			}
			for bi, a := range near {
				// Temporal stack: the die sits still for seconds after it
				// appears, so a per-pixel median over several frames keeps
				// the 3-5px pips while cancelling compression noise and a
				// hand crossing a single frame (survey: image stacking).
				var layers []*image.RGBA
				var r image.Rectangle
				for _, dt := range []int{500, 1000, 1500, 2000, 2500} {
					frame, err := capture.FrameAt(part.File, a.tick+dt)
					if err != nil {
						continue
					}
					if r.Empty() {
						// box in stream coords -> full-res, with margin
						fx := 1 / sx
						fy := 1 / sy
						r = image.Rect(
							int(float64(a.box.Min.X)*fx)-8, int(float64(a.box.Min.Y)*fy)-8,
							int(float64(a.box.Max.X)*fx)+8, int(float64(a.box.Max.Y)*fy)+8,
						).Intersect(frame.Bounds())
						if r.Dx() < 12 || r.Dy() < 12 {
							break
						}
					}
					layer := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
					for y := 0; y < r.Dy(); y++ {
						for x := 0; x < r.Dx(); x++ {
							layer.Set(x, y, frame.At(r.Min.X+x, r.Min.Y+y))
						}
					}
					layers = append(layers, layer)
				}
				if len(layers) < 3 {
					continue
				}
				crop := capture.MedianStack(layers)
				name := fmt.Sprintf("%s_i%d_b%d.png", m.ID, tn.Index, bi)
				f, err := os.Create(filepath.Join(*outDir, name))
				if err != nil {
					log.Fatal(err)
				}
				if err := png.Encode(f, crop); err != nil {
					f.Close()
					log.Fatal(err)
				}
				f.Close()
				dbl := 0
				if d1 == d2 {
					dbl = 1
				}
				fmt.Fprintf(lf, "%s,%s,%d,%d,%d,%d,%d,%s_i%d\n", name, m.ID, tn.Index, a.tick, d1, d2, dbl, m.ID, tn.Index)
				total++
			}
		}
	}
	fmt.Fprintf(os.Stderr, "%s: %d dice-box crops\n", m.ID, total)
}

// runCornercrops extracts the corner-CNN training dataset: frames sampled
// across a calibrated recording's span, downscaled, labeled with the
// manifest's (validated) corner coordinates — the board-localization
// survey's recommended learned upgrade, fed by the calibration campaign.
func runCornercrops(args []string) {
	fs := flag.NewFlagSet("cornercrops", flag.ExitOnError)
	manifest := fs.String("manifest", "", "Recording manifest JSON (required)")
	outDir := fs.String("out", "", "output directory (required)")
	n := fs.Int("n", 40, "frames sampled across the span")
	fs.Parse(args)
	if *manifest == "" || *outDir == "" {
		fs.Usage()
		os.Exit(2)
	}
	m := loadManifest(*manifest)
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatal(err)
	}
	lf, err := os.OpenFile(filepath.Join(*outDir, "labels.csv"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatal(err)
	}
	defer lf.Close()
	if st, _ := lf.Stat(); st.Size() == 0 {
		fmt.Fprintln(lf, "file,recording,part,tick_ms,w,h,tlx,tly,trx,try,brx,bry,blx,bly")
	}
	const dw, dh = 320, 180
	total := 0
	for pi, part := range m.Parts {
		span := part.Span.EndMs - part.Span.BeginMs
		if span <= 0 {
			continue
		}
		for k := 0; k < *n; k++ {
			tick := part.Span.BeginMs + span*(2*k+1)/(2*(*n))
			frame, err := capture.FrameAt(part.File, tick)
			if err != nil {
				continue
			}
			srcW := frame.Bounds().Dx()
			srcH := frame.Bounds().Dy()
			small := image.NewRGBA(image.Rect(0, 0, dw, dh))
			for y := 0; y < dh; y++ {
				for x := 0; x < dw; x++ {
					small.Set(x, y, frame.At(frame.Bounds().Min.X+x*srcW/dw, frame.Bounds().Min.Y+y*srcH/dh))
				}
			}
			name := fmt.Sprintf("%s_p%d_t%d.png", m.ID, pi, tick)
			f, err := os.Create(filepath.Join(*outDir, name))
			if err != nil {
				log.Fatal(err)
			}
			if err := png.Encode(f, small); err != nil {
				f.Close()
				log.Fatal(err)
			}
			f.Close()
			c := part.Calibration.Corners
			sx := float64(dw) / float64(srcW)
			sy := float64(dh) / float64(srcH)
			fmt.Fprintf(lf, "%s,%s,%d,%d,%d,%d,%.1f,%.1f,%.1f,%.1f,%.1f,%.1f,%.1f,%.1f\n",
				name, m.ID, pi, tick, dw, dh,
				c[0][0]*sx, c[0][1]*sy, c[1][0]*sx, c[1][1]*sy,
				c[2][0]*sx, c[2][1]*sy, c[3][0]*sx, c[3][1]*sy)
			total++
		}
	}
	fmt.Fprintf(os.Stderr, "%s: %d corner frames\n", m.ID, total)
}

func runAlign(args []string) {
	fs := flag.NewFlagSet("align", flag.ExitOnError)
	manifest := fs.String("manifest", "", "Recording manifest JSON (required)")
	writeManifest := fs.Bool("write-manifest", false, "write aligned per-turn ticks back into the manifest")
	cropsDir := fs.String("crops", "", "also extract labeled point crops into this directory")
	limitMs := fs.Int("limit-ms", 0, "stop each part this many ms after its span begins (0 = full span)")
	model := fs.String("model", "", "read boards with this learned point-reader weight file instead of the classical reader")
	fs.Parse(args)
	if *manifest == "" {
		fs.Usage()
		os.Exit(2)
	}
	m := loadManifest(*manifest)
	matBytes, err := os.ReadFile(m.Transcript)
	if err != nil {
		log.Fatalf("truth transcript: %v", err)
	}
	truth, err := matimport.Parse(string(matBytes))
	if err != nil {
		log.Fatalf("truth transcript: %v", err)
	}

	o := transcribe.DefaultRunOptions()
	o.LimitMs = *limitMs
	o.ModelPath = *model
	o.Log = os.Stderr
	events, err := transcribe.ReadEvents(".", m, o)
	if err != nil {
		log.Fatalf("read events: %v", err)
	}
	turns := align.TruthTurns(truth)
	assign := align.Align(turns, events)

	aligned := 0
	for k, turn := range turns {
		if assign[k] < 0 {
			fmt.Printf("turn %3d g%d %v %-24q  UNALIGNED\n", turn.Index, turn.Game, turn.Dice, turn.Notation)
			continue
		}
		aligned++
		fmt.Printf("turn %3d g%d %v %-24q @%dms\n", turn.Index, turn.Game, turn.Dice, turn.Notation, events[assign[k]].Tick)
	}
	fmt.Fprintf(os.Stderr, "aligned %d/%d turns over %d events\n", aligned, len(turns), len(events))

	if *writeManifest {
		m.Turns = nil
		for k := range turns {
			if assign[k] < 0 {
				continue
			}
			m.Turns = append(m.Turns, corpus.Turn{Index: turns[k].Index, Part: events[assign[k]].Part, TickMs: events[assign[k]].Tick})
		}
		data, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(*manifest, append(data, '\n'), 0o644); err != nil {
			log.Fatal(err)
		}
		fmt.Fprintf(os.Stderr, "wrote %d turn ticks to %s\n", len(m.Turns), *manifest)
	}
	if *cropsDir != "" {
		res, err := align.ExtractCrops(".", m, turns, assign, events, *cropsDir, 0.80)
		if err != nil {
			log.Fatalf("crops: %v", err)
		}
		fmt.Fprintf(os.Stderr, "wrote %d labeled crops from %d turns to %s\n", res.Crops, res.Turns, *cropsDir)
	}
}

func countPlies(m bg.Match) int {
	n := 0
	for _, g := range m.Games {
		n += len(g.Plies)
	}
	return n
}

func runDemo() {

	// A fixed sequence of rolls; the engine chooses each move.
	rolls := []struct {
		who  bg.Player
		dice bg.Dice
	}{
		{bg.P1, bg.Dice{3, 1}},
		{bg.P2, bg.Dice{6, 5}},
		{bg.P1, bg.Dice{5, 4}},
		{bg.P2, bg.Dice{3, 2}},
	}

	board := bg.StandardStart()
	policy := gate.Default()
	var plies []bg.Ply
	tick := 0

	for _, r := range rolls {
		tick += 1000
		ply, next, dec, err := playTurn(board, r.dice, r.who, tick)
		if err != nil {
			log.Fatalf("turn (%v %v): %v", r.who, r.dice, err)
		}
		outcome, _ := policy.Classify(dec)
		fmt.Fprintf(os.Stderr, "tick %5d  %-5s rolls %s → %-14s conf=%.2f  [%s]\n",
			tick, playerName(r.who), r.dice, dec.Top.Notation, dec.Confidence, outcome)
		plies = append(plies, ply)
		board = next
	}

	m := bg.Match{
		Length:  3,
		Players: [2]string{"Alice", "Bob"},
		Meta: []bg.MetaField{
			{Key: "Site", Value: "lazyBG engine demo"},
			{Key: "Player 1", Value: "Alice"},
			{Key: "Player 2", Value: "Bob"},
		},
		Games: []bg.Game{{Number: 1, Plies: plies}},
	}
	fmt.Fprint(os.Stdout, matexport.Write(m))
}

// playTurn asks the engine for the best move, "observes" its result (simulating
// perception), then recovers the move via boarddiff + fusion — exercising the
// real pipeline end-to-end. Returns the auto-fill ply, the resulting board, and
// the decision.
func playTurn(board bg.Board, dice bg.Dice, who bg.Player, tick int) (bg.Ply, bg.Board, cue.MoveDecision, error) {
	pre := bg.Position{Board: board, Dice: dice, PlayerOnRoll: who}
	moves, err := engine.LegalMoves(pre)
	if err != nil {
		return bg.Ply{}, board, cue.MoveDecision{}, err
	}
	if len(moves) == 0 { // dance
		ply := bg.Ply{Player: who, Dice: dice, CannotMove: true, Tick: tick}
		return ply, board, cue.MoveDecision{Player: who, Tick: tick}, nil
	}

	played := moves[0]                 // a strong player plays the engine's best move
	observed := observe(played.Result) // stand-in for the board-state reader
	dec, err := boarddiff.Decide(pre, observed, &dice, tick, fusion.DefaultWeights())
	if err != nil {
		return bg.Ply{}, board, cue.MoveDecision{}, err
	}
	ply := bg.Ply{
		Player:     who,
		Dice:       dice,
		Notation:   dec.Top.Notation,
		Tick:       tick,
		Confidence: dec.Confidence,
	}
	return ply, played.Result, dec, nil
}

// observe fabricates a clean, fully-confident reading of a board (stand-in for
// the board-state reader).
func observe(b bg.Board) perceive.ObservedBoard {
	var ob perceive.ObservedBoard
	for p := 1; p <= 24; p++ {
		c := b.Pts[p]
		if c.N == 0 {
			ob.Points[p] = perceive.PointObs{Confidence: 1}
			continue
		}
		side := perceive.A
		if c.Owner == bg.P2 {
			side = perceive.B
		}
		ob.Points[p] = perceive.PointObs{Count: c.N, Side: side, Confidence: 1}
	}
	return ob
}

func playerName(p bg.Player) string {
	if p == bg.P1 {
		return "Alice"
	}
	return "Bob"
}
