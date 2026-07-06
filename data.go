// Package lazybg is the module root. Its sole job is to embed the gnubg engine
// data so the shipped binary is a single, fully-offline artifact (CLAUDE.md §8,
// docs/architecture.md §6). The embed lives here, at the module root, because
// //go:embed can only reach files at or below its own directory — and data/ is
// a top-level directory.
package lazybg

import "embed"

// DataFS carries the engine data under a top-level data/ directory (weights,
// one/two-sided bearoff DBs, match-equity tables), ready to hand to
// engine.Init / gnubg.Init.
//
//go:embed all:data
var DataFS embed.FS
