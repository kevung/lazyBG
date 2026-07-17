//go:build desktop

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"lazybg/internal/bg"
	"lazybg/internal/session"
)

// App is the Wails binding surface: parameter/return shaping only, no
// business logic (ADR-0003). The session service does the real work.
type App struct {
	ctx context.Context

	mu        sync.Mutex
	videoPath string
	svc       *session.Service
}

func NewApp() *App {
	return &App{svc: session.New()}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) currentVideoPath() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.videoPath
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
	a.svc.SetVideoPos(tickMs)
}

// EnterDice records the observed roll and returns the ranked candidates —
// or, on a dance, the already-recorded Cannot Move ply (no candidate step).
func (a *App) EnterDice(d1, d2, tickMs int) (session.DiceResult, error) {
	return a.svc.EnterDiceAt(d1, d2, tickMs)
}

// Confirm applies the candidate at index, stamped with the video tick (ms).
func (a *App) Confirm(index, tickMs int) (session.PlyView, error) {
	return a.svc.Confirm(index, tickMs)
}

// ConfirmFlag confirms AND opens a human-flagged Review Item (Shift+Space).
func (a *App) ConfirmFlag(index, tickMs int) (session.PlyView, error) {
	return a.svc.ConfirmFlag(index, tickMs)
}

// Override records a free-entry move, bypassing the candidate list
// (ADR-0001). Empty notation records a Cannot Move.
func (a *App) Override(notation string, tickMs int) (session.PlyView, error) {
	return a.svc.Override(notation, tickMs)
}

// ReviewItems returns the open review-queue entries.
func (a *App) ReviewItems() []session.ReviewItemView {
	return a.svc.ReviewItems()
}

// BoardState returns the current reconstructed board.
func (a *App) BoardState() bg.Board {
	return a.svc.Board()
}

// BoardAt returns the reconstructed board after the ply at seq (-1 = start).
func (a *App) BoardAt(seq int) (bg.Board, error) {
	return a.svc.BoardAt(seq)
}

// PendingGameEnd returns the detected (unconfirmed) game end, or nil.
func (a *App) PendingGameEnd() *session.GameEndProposal {
	return a.svc.PendingGameEnd()
}

// ConfirmGameEnd closes the game with the (possibly corrected) result.
func (a *App) ConfirmGameEnd(winner, points int) (session.GameEndResult, error) {
	return a.svc.ConfirmGameEnd(winner, points)
}

// Score returns the running match score.
func (a *App) Score() [2]int {
	return a.svc.Score()
}

// CubeActions returns the cube actions available to the player on roll.
func (a *App) CubeActions() []string {
	return a.svc.CubeActions()
}

// EnterCube records a cube action (double/take/drop) at the video tick.
func (a *App) EnterCube(action string, tickMs int) (session.PlyView, error) {
	return a.svc.EnterCube(action, tickMs)
}

// SetTurnPlayer declares who the pending turn belongs to (0 or 1).
func (a *App) SetTurnPlayer(player int) error {
	return a.svc.SetTurnPlayer(player)
}

// Moves returns the move list so far.
func (a *App) Moves() []session.PlyView {
	return a.svc.Moves()
}

// OnRoll returns the player the next turn belongs to (0 or 1).
func (a *App) OnRoll() int {
	return a.svc.OnRoll()
}
