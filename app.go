package main

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	// "fmt"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

func (a *App) SaveDatabaseDialog() (string, error) {
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:                "New Database File",
		Filters:              []runtime.FileFilter{{DisplayName: "Database Files (*.db)", Pattern: "*.db"}},
		CanCreateDirectories: true,
	})
}

func (a *App) OpenDatabaseDialog() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Open Database File",
		Filters: []runtime.FileFilter{{DisplayName: "Database Files (*.db)", Pattern: "*.db"}},
	})
}

func (a *App) OpenTranscriptionDialog() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Open Match Transcription",
		Filters: []runtime.FileFilter{
			{DisplayName: "Transcription Files", Pattern: "*.lbg;*.txt"},
			{DisplayName: "lazyBG Files (*.lbg)", Pattern: "*.lbg"},
			{DisplayName: "Match Text Files (*.txt)", Pattern: "*.txt"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
}

func (a *App) SaveTranscriptionDialog() (string, error) {
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:                "Save Match Transcription",
		DefaultFilename:      "match.lbg",
		Filters:              []runtime.FileFilter{{DisplayName: "lazyBG Files (*.lbg)", Pattern: "*.lbg"}},
		CanCreateDirectories: true,
	})
}

func (a *App) ReadTextFile(filePath string) (string, error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (a *App) WriteTextFile(filePath string, content string) error {
	return ioutil.WriteFile(filePath, []byte(content), 0644)
}

func (a *App) DeleteFile(filePath string) error {
	err := os.Remove(filePath)
	if err != nil {
		return err
	}
	return nil
}

type FileDialogResponse struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
	Error    string `json:"error,omitempty"` // Optional field to capture any errors
}

func (a *App) OpenPositionDialog() (*FileDialogResponse, error) {
	// Open the file dialog
	filePath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Import Position File",
		Filters: []runtime.FileFilter{{DisplayName: "Position Files (*.txt)", Pattern: "*.txt"}},
	})

	if err != nil {
		return &FileDialogResponse{Error: err.Error()}, err
	}

	if filePath == "" {
		return &FileDialogResponse{Error: "No file selected"}, nil
	}

	// Read the file content
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return &FileDialogResponse{Error: err.Error()}, err
	}

	return &FileDialogResponse{
		FilePath: filePath,
		Content:  string(content),
	}, nil
}

func (a *App) ShowAlert(message string) {
	runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:    runtime.InfoDialog,
		Title:   "Alert",
		Message: message,
	})
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}
