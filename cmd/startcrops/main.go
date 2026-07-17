// Dev tool: extract labeled per-point crops from a KNOWN start position (no
// .mat needed — the opening layout is fixed a priori). Reuses align.ExtractCrops
// for format parity. Board layout is passed as a spec string:
//   "A:p=n,... B:p=n,..."  (A=CheckerA color=P1, B=CheckerB=P2)
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"lazybg/internal/align"
	"lazybg/internal/bg"
	"lazybg/internal/corpus"
	"lazybg/internal/perceive"
	"lazybg/internal/transcribe"
)

func parseBoard(spec string) bg.Board {
	var b bg.Board
	for _, part := range strings.Fields(spec) {
		who := bg.P1
		body := part
		if strings.HasPrefix(part, "A:") { who = bg.P1; body = part[2:] }
		if strings.HasPrefix(part, "B:") { who = bg.P2; body = part[2:] }
		for _, pc := range strings.Split(body, ",") {
			if pc == "" { continue }
			kv := strings.Split(pc, "=")
			p, _ := strconv.Atoi(kv[0])
			n, _ := strconv.Atoi(kv[1])
			b.Pts[p] = bg.Point{N: n, Owner: who}
		}
	}
	return b
}

func obsOf(b bg.Board) perceive.ObservedBoard {
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

func main() {
	manifest := flag.String("manifest", "", "tuned manifest json")
	out := flag.String("out", "", "output crops dir")
	ticks := flag.String("ticks", "", "comma-separated absolute tick ms")
	board := flag.String("board", "", "layout spec")
	flag.Parse()

	data, err := os.ReadFile(*manifest)
	if err != nil { log.Fatal(err) }
	m, err := corpus.Load(data)
	if err != nil { log.Fatal(err) }
	b := parseBoard(*board)
	// sanity: 15+15
	if b.Checkers(bg.P1) != 15 || b.Checkers(bg.P2) != 15 {
		log.Fatalf("board not 15+15: A=%d B=%d", b.Checkers(bg.P1), b.Checkers(bg.P2))
	}
	ob := obsOf(b)

	var turns []align.Turn
	var events []transcribe.Event
	var assign []int
	for i, ts := range strings.Split(*ticks, ",") {
		tick, _ := strconv.Atoi(ts)
		turns = append(turns, align.Turn{Board: b, Game: 1, Index: i})
		events = append(events, transcribe.Event{Tick: tick, Obs: ob, Part: 0})
		assign = append(assign, i)
	}
	res, err := align.ExtractCrops(".", m, turns, assign, events, *out, 0.0)
	if err != nil { log.Fatal(err) }
	fmt.Printf("turns=%d crops=%d -> %s\n", res.Turns, res.Crops, *out)
}
