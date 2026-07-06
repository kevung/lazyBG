package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kevung/gnubgparser"
	"xg2mat/convert"
)

func main() {
	outExt := flag.String("ext", ".mat", "output extension")
	validate := flag.Bool("validate", true, "re-parse output with gnubgparser and cross-check")
	write := flag.Bool("write", true, "write output files next to inputs")
	flag.Parse()

	var failures int
	for _, in := range flag.Args() {
		res, err := convert.FromFile(in)
		if err != nil {
			fmt.Printf("FAIL  %s: %v\n", in, err)
			failures++
			continue
		}

		out := strings.TrimSuffix(in, ".xg") + *outExt
		status := "ok"
		var notes []string

		if *validate {
			if err := crossCheck(res); err != nil {
				status = "MISMATCH"
				notes = append(notes, err.Error())
				failures++
			}
		}
		if len(res.Warnings) > 0 {
			notes = append(notes, fmt.Sprintf("%d warn", len(res.Warnings)))
		}

		if *write {
			if err := os.WriteFile(out, []byte(res.MAT), 0o644); err != nil {
				fmt.Printf("FAIL  %s: write: %v\n", in, err)
				failures++
				continue
			}
		}

		fmt.Printf("%-8s %-3d games  %dp  %-22s vs %-22s  %s\n",
			status, res.Games, res.Length, res.Player1, res.Player2, strings.Join(notes, "; "))
		for _, w := range res.Warnings {
			fmt.Printf("    ! %s\n", w)
		}
	}
	if failures > 0 {
		os.Exit(1)
	}
}

func crossCheck(res *convert.Result) error {
	m, err := gnubgparser.ParseMAT(strings.NewReader(res.MAT))
	if err != nil {
		return fmt.Errorf("reparse: %w", err)
	}
	if len(m.Games) != res.Games {
		return fmt.Errorf("reparse games=%d want=%d", len(m.Games), res.Games)
	}
	if int32(m.Metadata.MatchLength) != res.Length {
		return fmt.Errorf("reparse length=%d want=%d", m.Metadata.MatchLength, res.Length)
	}
	return nil
}
