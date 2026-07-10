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
	"log"
	"os"

	"encoding/json"

	"lazybg"
	"lazybg/internal/align"
	"lazybg/internal/bg"
	"lazybg/internal/corpus"
	"lazybg/internal/cue"
	"lazybg/internal/engine"
	"lazybg/internal/eval"
	"lazybg/internal/fusion"
	"lazybg/internal/gate"
	"lazybg/internal/matexport"
	"lazybg/internal/matimport"
	"lazybg/internal/perceive"
	"lazybg/internal/perceive/boarddiff"
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

func runPipeline(manifest string, limitMs int) (transcribe.Outcome, corpus.Manifest) {
	m := loadManifest(manifest)
	o := transcribe.DefaultRunOptions()
	o.LimitMs = limitMs
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
	fs.Parse(args)
	if *manifest == "" {
		fs.Usage()
		os.Exit(2)
	}
	out, _ := runPipeline(*manifest, *limitMs)
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
	fs.Parse(args)
	if *manifest == "" {
		fs.Usage()
		os.Exit(2)
	}
	out, m := runPipeline(*manifest, *limitMs)
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
	fmt.Printf("games:            %d produced / %d truth\n", s.GamesOut, s.GamesTruth)
	fmt.Printf("checker plies:    %d produced / %d truth\n", s.OutCheckerPlies, s.TruthCheckerPlies)
	fmt.Printf("matched:          %d\n", s.Matched)
	fmt.Printf("auto-filled:      %d (errors %d, rate %.3f)\n", s.AutoFilled, s.AutoFillErrors(), s.ErrorRate())
	fmt.Printf("coverage:         %.3f\n", s.Coverage())
	fmt.Printf("review rate:      %.3f\n", s.ReviewRate())
	fmt.Printf("truth cube plays: %d (not yet perceived)\n", s.TruthCubeActions)
}

func runAlign(args []string) {
	fs := flag.NewFlagSet("align", flag.ExitOnError)
	manifest := fs.String("manifest", "", "Recording manifest JSON (required)")
	writeManifest := fs.Bool("write-manifest", false, "write aligned per-turn ticks back into the manifest")
	cropsDir := fs.String("crops", "", "also extract labeled point crops into this directory")
	limitMs := fs.Int("limit-ms", 0, "stop each part this many ms after its span begins (0 = full span)")
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
			m.Turns = append(m.Turns, corpus.Turn{Index: turns[k].Index, Part: 0, TickMs: events[assign[k]].Tick})
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
