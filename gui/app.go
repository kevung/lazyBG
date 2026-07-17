//go:build desktop

package main

import (
	"context"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

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

// OpenVideoDialog lets the user pick a video file and returns the URL the
// frontend's <video> element should load (served by the media handler).
// An empty string means the user cancelled.
func (a *App) OpenVideoDialog() (string, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Open match video",
		Filters: []runtime.FileFilter{
			{DisplayName: "Videos", Pattern: "*.mp4;*.mkv;*.webm;*.avi;*.mov"},
			{DisplayName: "All files", Pattern: "*"},
		},
	})
	if err != nil || path == "" {
		return "", err
	}
	a.mu.Lock()
	a.videoPath = path
	a.svc = session.New() // fresh session per video (persistence is ticket #13)
	a.mu.Unlock()
	return "/media/current", nil
}

// EnterDice records the observed roll and returns the ranked candidates.
func (a *App) EnterDice(d1, d2 int) ([]session.Candidate, error) {
	return a.svc.EnterDice(d1, d2)
}

// Confirm applies the candidate at index, stamped with the video tick (ms).
func (a *App) Confirm(index, tickMs int) (session.PlyView, error) {
	return a.svc.Confirm(index, tickMs)
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
