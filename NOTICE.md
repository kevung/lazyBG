# Third-party notices

lazyBG is licensed under the MIT License (see [`LICENSE`](LICENSE)), Copyright (c) 2025-2026
Kévin Unger. That license covers lazyBG's own source: `cmd/`, `internal/`, `ml/`, `tools/`,
`docs/`, and `testdata/`.

lazyBG also bundles and redistributes third-party work, listed below. lazyBG is **not** a fork of
any of these projects and does not track their development; the code and data below were taken
deliberately and are maintained here independently.

---

## 1. The backgammon engine — `gnubg/`

The Go engine (position evaluation, neural nets, bearoff databases, match equity tables, position
keys) is a **port of [GNU Backgammon](https://www.gnu.org/software/gnubg/)**.

lazyBG did not port it from the C original. It was taken from
**[bgweb-api](https://github.com/foochu/bgweb-api)** by **Rami Keränen** (`foochu`), which
published the Go port under the MIT License, Copyright (c) 2022 Rami Keränen. That notice is
preserved verbatim in [`gnubg/LICENSE`](gnubg/LICENSE) as the MIT terms require.

lazyBG's git history begins with bgweb-api's history — the repository was originally created as a
GitHub fork of it, and has since been detached into a standalone project. Commits authored by
Rami Keränen remain in the history and are the origin of this directory.

Changes made in lazyBG: the package was adapted to lazyBG's module path, its data loading was
abstracted behind an `fs.FS`, and it is exercised by lazyBG's own tests. The evaluation logic is
otherwise upstream's.

## 2. Engine data — `data/`

### `data/gnubg.weights`, `data/gnubg_os0.bd`, `data/gnubg_ts0.bd`

Neural-network weights and bearoff databases produced by and distributed with the **GNU Backgammon**
project. These are binary artifacts and carry no embedded notice; their provenance is recorded here.

### `data/met/*.xml`

Match equity tables distributed as part of GNU Backgammon:

- `Kazaross-XG2.xml` — Copyright (C) 2011 Neil Kazaross; transcribed for GNU Backgammon by
  Michael Petch.
- `Rockwell-Kazaross.xml` — Copyright (C) 2008-2010 David Rockwell and Neil Kazaross.

Each file carries an all-permissive notice in its own header — copying and distribution in any
medium are permitted without royalty provided the copyright notice and that notice are preserved.
They are preserved, unmodified, in the files themselves.
