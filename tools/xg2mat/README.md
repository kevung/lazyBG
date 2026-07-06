# xg2mat — eXtreme Gammon (.xg) → Jellyfish (.mat) converter

Converts eXtreme Gammon match files (`.xg`) into the canonical Jellyfish
`.mat` / `.txt` transcription format (readable by gnubg, XG, BGBlitz). Used to
build ground-truth transcriptions for the lazyBG corpus without opening each
match by hand in ExtremeGammon.

`.mat`, `.txt` and "Jellyfish" all denote the same text format — one output
file, the extension is your choice (`-ext`).

## Usage

```bash
go build -o xg2mat .
./xg2mat path/to/match.xg ...          # writes match.mat next to each input
./xg2mat -write=false match.xg         # dry run: validate only, no file written
./xg2mat -ext=.txt match.xg            # write .txt instead of .mat
./xg2mat -validate=false match.xg      # skip the gnubgparser reparse cross-check
```

Each input produces a status line: `ok / MISMATCH / FAIL`, game count, match
length, and both player names. Any per-move warnings are printed with `!`.

## Correctness

The converter is validated two ways, both on by default:

1. **Board replay (ground truth).** For every checker move, the parsed
   from/to/hit is applied to the move's start position (`PositionI`) and
   compared against XG's recorded end position (`PositionEnd`). All 36 corpus
   matches reconcile with **zero** mismatches — i.e. the move notation and hit
   markers reproduce XG's own board exactly, move by move.
2. **Reparse cross-check.** The emitted `.mat` is parsed back with the
   independent `gnubgparser` MAT reader; game count and match length must agree.

## XG format notes (hard-won — read before touching the decode)

- `HeaderMatchEntry` → players, match length (⚠ trust the file: one match named
  `…7p.xg` is internally a **13-point** match), event/site/round/date.
- `MoveEntry.Moves` is `[from,to, …]` (up to 4 pairs) in the **mover's own
  perspective**, **0-indexed**: point value `v` → board point `v+1`; `24` = bar;
  a **negative** `to` (`-1 … -6`, or `from==to`) = **bear-off** (`/0`).
- `MoveEntry.PositionI` is **absolute** (player1 positive, player2 negative,
  index 1..24 = absolute points). `MoveEntry.PositionEnd` is in the **mover's**
  perspective (mover positive). *The two arrays use different frames* — this is
  the single biggest trap. Hit detection reads `PositionI`, so mover points must
  be mirrored to absolute (`24-v` for player2) with the mover's sign.
- `MoveEntry.Played == false` (with `Moves` all-zero) = a phantom end-of-game
  placeholder → skipped. A genuine dance (rolled, no legal move) is
  `Played == true` with `Moves` all `-1` → rendered as bare dice (`65:`).
- `CubeEntry`: `Double==1` = a double offered by `ActiveP`; the response is in
  `Take` (`1` = take, `0` = drop → game ends). `Double ∈ {-2,-1,0}` = no cube
  action (initial / Crawford / rolled). Cube display value is the *offered*
  cube; points won come from `FooterGameEntry.PointsWon`.
- `FooterGameEntry.Winner`: `1` = player1, `-1` = player2 (validated against the
  next game's score delta).

## Rebuilding / re-vendoring

Dependencies (`github.com/kevung/xgparser`, `github.com/kevung/gnubgparser` —
Kévin Unger's own libraries) are **vendored** under `vendor/`, so `go build`
works fully offline with no network access.

To re-vendor from fresh local checkouts, point the `replace` directives in
`go.mod` at them and run `go mod vendor`. Note: the upstream `xgparser`'s
`xgwrite.go` (XG *writer*, unused here) references fields that no longer exist
and won't compile — add a `//go:build ignore` line at its top before vendoring
(the writer is not needed for reading/conversion, and `go mod vendor` then
excludes it cleanly).
