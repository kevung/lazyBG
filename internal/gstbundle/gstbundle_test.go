package gstbundle

import (
	"os"
	"testing"
)

func TestPluginDirIsBesideExecutable(t *testing.T) {
	got := PluginDir("/opt/lazybg/lazybg-gui")
	want := "/opt/lazybg/gstreamer-1.0"
	if got != want {
		t.Fatalf("PluginDir = %q, want %q", got, want)
	}
}

func TestPrepend(t *testing.T) {
	sep := string(os.PathListSeparator)
	if got := Prepend("/a", ""); got != "/a" {
		t.Errorf("empty existing: got %q", got)
	}
	if got := Prepend("/a", "/b"); got != "/a"+sep+"/b" {
		t.Errorf("prepend: got %q", got)
	}
}
