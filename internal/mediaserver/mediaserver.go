// Package mediaserver serves the currently-open match video over a dedicated
// loopback HTTP server so the GUI's HTML5 <video> can play and seek it.
//
// Why a standalone server rather than the Wails asset-server Handler: in
// `wails dev` the asset pipeline is delegated to the Vite dev server, whose SPA
// fallback answers /media/current with index.html (HTTP 200), so the custom
// Go handler is never reached and the <video> receives HTML — nothing plays
// (ADR-0004). An absolute http://127.0.0.1:<port>/ URL bypasses Vite entirely
// and behaves identically in dev and production. http.ServeFile handles Range
// requests, so scrubbing works.
package mediaserver

import (
	"net"
	"net/http"
	"sync"
)

// Server serves the current video file at /media/current on a loopback port.
// The served path is swapped with SetPath as the user opens videos; only paths
// the app hands us are ever served.
type Server struct {
	ln  net.Listener
	srv *http.Server

	mu   sync.RWMutex
	path string
}

// New starts a media server on an OS-assigned loopback port (an ephemeral port
// avoids collisions with other dev instances) and serves in the background.
func New() (*Server, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s := &Server{ln: ln}
	mux := http.NewServeMux()
	mux.HandleFunc("/media/current", s.serveCurrent)
	s.srv = &http.Server{Handler: mux}
	go s.srv.Serve(ln)
	return s, nil
}

// SetPath swaps the file served at /media/current (empty = no video open).
func (s *Server) SetPath(path string) {
	s.mu.Lock()
	s.path = path
	s.mu.Unlock()
}

func (s *Server) currentPath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.path
}

// BaseURL is the server origin, e.g. http://127.0.0.1:54321.
func (s *Server) BaseURL() string {
	return "http://" + s.ln.Addr().String()
}

// MediaURL is the absolute URL the <video> element points at.
func (s *Server) MediaURL() string {
	return s.BaseURL() + "/media/current"
}

// Close stops the server.
func (s *Server) Close() error {
	return s.srv.Close()
}

func (s *Server) serveCurrent(w http.ResponseWriter, r *http.Request) {
	// The page is served from another origin and draws the frame onto a
	// <canvas> (calibration, frame snapshot); a permissive CORS header keeps the
	// canvas from tainting so pixel reads keep working (pair with the video
	// element's crossorigin="anonymous").
	w.Header().Set("Access-Control-Allow-Origin", "*")
	path := s.currentPath()
	if path == "" {
		http.Error(w, "no video open", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, path)
}
