//go:build lazybggui

package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	lazybg "lazybg"
	"lazybg/internal/bg"
	"lazybg/internal/perceive/pointnet"
	"lazybg/internal/session"
)

// App is the Wails binding surface: parameter/return shaping only, no
// business logic (ADR-0003). The session service does the real work.
type App struct {
	ctx context.Context

	mu        sync.Mutex
	videoPath string
	svc       *session.Service

	// reader is the embedded learned board reader, shared across sessions and
	// wired into each on open so the board-diff cue re-weights candidates
	// (issue #23). Nil when the model fails to load — sessions run equity-only.
	reader session.BoardReader
}

func NewApp() *App {
	a := &App{svc: session.New(), reader: loadBoardReader()}
	a.svc.EnableVideoObservation(a.reader)
	return a
}

// loadBoardReader loads the embedded learned point reader. It beats the
// classical baseline on blind reads (CLAUDE.md §2) and needs only board
// geometry — no declared checker colors — so it is the GUI's default observer.
// A load failure is non-fatal: the session falls back to equity-only ranking.
func loadBoardReader() session.BoardReader {
	raw, err := lazybg.DataFS.ReadFile("data/models/pointreader.bin")
	if err != nil {
		log.Printf("board reader unavailable: %v (ranking will be equity-only)", err)
		return nil
	}
	net, err := pointnet.LoadBytes(raw)
	if err != nil {
		log.Printf("board reader unavailable: %v (ranking will be equity-only)", err)
		return nil
	}
	return pointnet.Reader{Net: net}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) currentVideoPath() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.videoPath
}

// service returns the current session under the lock: OpenVideoDialog swaps
// a.svc, and Wails dispatches bound calls on separate goroutines.
func (a *App) service() *session.Service {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.svc
}

// OpenResult is what the frontend needs after opening a video: the media URL,
// the resumed state (if a .lbg already existed next to the video), and any
// warning (missing/substituted video fingerprint).
type OpenResult struct {
	VideoURL   string            `json:"videoUrl"`
	LBGPath    string            `json:"lbgPath"`
	Resumed    bool              `json:"resumed"`
	Moves      []session.PlyView `json:"moves"`
	LastTickMs int               `json:"lastTickMs"`
	OnRoll     int               `json:"onRoll"`
	Warning    string            `json:"warning,omitempty"`
}

// OpenVideoDialog lets the user pick a video file, then creates — or resumes,
// if one already exists next to the video — the .lbg session (autosaved on
// every confirm; session-format-spec). A nil result means the user cancelled.
func (a *App) OpenVideoDialog() (*OpenResult, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Open match video",
		Filters: []runtime.FileFilter{
			{DisplayName: "Videos", Pattern: "*.mp4;*.mkv;*.webm;*.avi;*.mov"},
			{DisplayName: "All files", Pattern: "*"},
		},
	})
	if err != nil || path == "" {
		return nil, err
	}

	lbgPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".lbg"
	var (
		svc     *session.Service
		warning string
		resumed bool
	)
	if _, statErr := os.Stat(lbgPath); statErr == nil {
		svc, warning, err = session.Open(lbgPath)
		resumed = true
	} else {
		svc, warning, err = session.Create(lbgPath, path, "")
	}
	if err != nil {
		return nil, err
	}

	svc.EnableVideoObservation(a.reader)
	go computeCandidateTicks(svc) // segmentation for Tab nav (issue #23), off the UI thread

	a.mu.Lock()
	a.videoPath = path
	a.svc = svc
	a.mu.Unlock()

	return &OpenResult{
		VideoURL:   "/media/current",
		LBGPath:    lbgPath,
		Resumed:    resumed,
		Moves:      svc.Moves(),
		LastTickMs: svc.LastVideoPos(),
		OnRoll:     svc.OnRoll(),
		Warning:    warning,
	}, nil
}

// SetVideoPos persists the last-worked video position (resume state).
func (a *App) SetVideoPos(tickMs int) {
	a.service().SetVideoPos(tickMs)
}

// EnterDice records the observed roll and returns the ranked candidates —
// or, on a dance, the already-recorded Cannot Move ply (no candidate step).
func (a *App) EnterDice(d1, d2, tickMs int) (session.DiceResult, error) {
	return a.service().EnterDiceAt(d1, d2, tickMs)
}

// Confirm applies the candidate at index, stamped with the video tick (ms).
func (a *App) Confirm(index, tickMs int) (session.PlyView, error) {
	return a.service().Confirm(index, tickMs)
}

// ConfirmFlag confirms AND opens a human-flagged Review Item (Shift+Space).
func (a *App) ConfirmFlag(index, tickMs int) (session.PlyView, error) {
	return a.service().ConfirmFlag(index, tickMs)
}

// Override records a free-entry move, bypassing the candidate list
// (ADR-0001). Empty notation records a Cannot Move.
func (a *App) Override(notation string, tickMs int) (session.PlyView, error) {
	return a.service().Override(notation, tickMs)
}

// ReviewItems returns the open review-queue entries.
func (a *App) ReviewItems() []session.ReviewItemView {
	return a.service().ReviewItems()
}

// BoardState returns the current reconstructed board.
func (a *App) BoardState() bg.Board {
	return a.service().Board()
}

// BoardAt returns the reconstructed board after the ply at seq (-1 = start).
func (a *App) BoardAt(seq int) (bg.Board, error) {
	return a.service().BoardAt(seq)
}

// ExportDialog asks where to save the .mat and writes both projections
// (.mat + .manifest.json) from the current session state. Returns the paths
// written, or empty strings if the user cancelled.
func (a *App) ExportDialog() ([2]string, error) {
	matPath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export .mat",
		DefaultFilename: "match.mat",
		Filters:         []runtime.FileFilter{{DisplayName: "Jellyfish match", Pattern: "*.mat"}},
	})
	if err != nil || matPath == "" {
		return [2]string{}, err
	}
	if err := a.service().ExportMat(matPath); err != nil {
		return [2]string{}, err
	}
	manPath := strings.TrimSuffix(matPath, filepath.Ext(matPath)) + ".manifest.json"
	if err := a.service().ExportManifest(manPath, matPath); err != nil {
		return [2]string{}, err
	}
	return [2]string{matPath, manPath}, nil
}

// GetSetup returns the current session setup (pre-filled form).
func (a *App) GetSetup() session.Setup {
	return a.service().GetSetup()
}

// SaveSetup stores the setup; recorded turns are never touched. A fresh
// calibration re-runs segmentation for Tab navigation (issue #23) in the
// background.
func (a *App) SaveSetup(setup session.Setup) error {
	svc := a.service()
	if err := svc.SaveSetup(setup); err != nil {
		return err
	}
	go computeCandidateTicks(svc)
	return nil
}

// CandidateTicks returns the segmentation-proposed navigation ticks, or an
// empty list while the scan is still running (or the session is uncalibrated).
func (a *App) CandidateTicks() []int {
	return a.service().CandidateTicks()
}

// computeCandidateTicks runs the (slow) stable-window scan and logs the outcome.
func computeCandidateTicks(svc *session.Service) {
	if n, err := svc.ComputeCandidateTicks(); err != nil {
		log.Printf("candidate-tick segmentation: %v", err)
	} else if n > 0 {
		log.Printf("candidate-tick segmentation: %d ticks", n)
	}
}

// SetupDone reports whether the blocking setup step is complete.
func (a *App) SetupDone() bool {
	return a.service().SetupDone()
}

// CandidatesFor re-opens the entry flow at a past turn (edit mode).
func (a *App) CandidatesFor(seq, d1, d2 int) ([]session.Candidate, error) {
	return a.service().CandidatesFor(seq, d1, d2)
}

// ReplaceTurn edits a recorded turn; downstream turns re-validate and any
// now-illegal ones join the review queue (never deleted).
func (a *App) ReplaceTurn(seq, d1, d2 int, notation string) error {
	return a.service().ReplaceTurn(seq, d1, d2, notation)
}

// DeleteTurn removes a recorded turn and re-validates the chain.
func (a *App) DeleteTurn(seq int) error {
	return a.service().DeleteTurn(seq)
}

// PendingGameEnd returns the detected (unconfirmed) game end, or nil.
func (a *App) PendingGameEnd() *session.GameEndProposal {
	return a.service().PendingGameEnd()
}

// ConfirmGameEnd closes the game with the (possibly corrected) result.
func (a *App) ConfirmGameEnd(winner, points int) (session.GameEndResult, error) {
	return a.service().ConfirmGameEnd(winner, points)
}

// Score returns the running match score.
func (a *App) Score() [2]int {
	return a.service().Score()
}

// MarkReviewed resolves a turn's open Review Items without changing it.
func (a *App) MarkReviewed(seq int) error {
	return a.service().MarkReviewed(seq)
}

// CubeActions returns the cube actions available to the player on roll.
func (a *App) CubeActions() []string {
	return a.service().CubeActions()
}

// EnterCube records a cube action (double/take/drop) at the video tick.
func (a *App) EnterCube(action string, tickMs int) (session.PlyView, error) {
	return a.service().EnterCube(action, tickMs)
}

// SetTurnPlayer declares who the pending turn belongs to (0 or 1).
func (a *App) SetTurnPlayer(player int) error {
	return a.service().SetTurnPlayer(player)
}

// Moves returns the move list so far.
func (a *App) Moves() []session.PlyView {
	return a.service().Moves()
}

// OnRoll returns the player the next turn belongs to (0 or 1).
func (a *App) OnRoll() int {
	return a.service().OnRoll()
}
