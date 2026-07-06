module xg2mat

go 1.21

require (
	github.com/kevung/gnubgparser v0.0.0
	github.com/kevung/xgparser v0.0.0
)

// The XG/MAT parsing libraries (Kévin Unger's own) are vendored under vendor/,
// so builds need no network. These replace paths are only used to RE-vendor
// (`go mod vendor`); adjust them to your local checkouts of the libraries.
replace github.com/kevung/xgparser => ../../tmp/blunderDB/xgparser

replace github.com/kevung/gnubgparser => ../../tmp/blunderDB/gnubgparser
