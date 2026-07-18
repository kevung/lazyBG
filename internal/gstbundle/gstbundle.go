// Package gstbundle helps the Linux build stay self-contained for video
// playback. WebKitGTK decodes the <video> element via system GStreamer plugins;
// H.264 needs gst-libav, which is not installed on every machine (ADR-0004).
// When the app ships its own plugins next to the executable, this package
// points GStreamer at them so playback works without a system codec.
//
// The pure helpers here are unit-tested; the gui wires them in main.go. When no
// bundled plugin directory exists (e.g. in dev, or an unbundled build) the hook
// is a no-op and the system GStreamer is used unchanged.
package gstbundle

import (
	"os"
	"path/filepath"
)

// PluginDir returns the bundled-plugin directory for a given executable path:
// <exeDir>/gstreamer-1.0.
func PluginDir(exePath string) string {
	return filepath.Join(filepath.Dir(exePath), "gstreamer-1.0")
}

// Prepend puts dir at the front of an existing GST_PLUGIN_PATH-style value so a
// bundled plugin wins over a system one, without discarding the system path.
func Prepend(dir, existing string) string {
	if existing == "" {
		return dir
	}
	return dir + string(os.PathListSeparator) + existing
}
