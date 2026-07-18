// Package proxy builds a webview-playable copy (a Playback Proxy, domain-model
// §2) of a match video when the OS webview cannot decode the original's
// codec/container. It is invoked lazily — only after the <video> reports it
// cannot play the original (ADR-0004).
//
// Timeline fidelity is the hard constraint: the Tick the user stamps from the
// proxy must equal the Tick on the original Capture (the automatic pipeline
// decodes the original). So we prefer stream-copy remux (exact), transcode only
// as a fallback, never resample frame rate nor pre-seek, and verify duration
// parity after building.
package proxy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"lazybg/internal/capture"
)

// FFprobeBin is the ffprobe executable (bundled alongside ffmpeg in the shipped
// app; tests use PATH).
var FFprobeBin = "ffprobe"

// fingerprintPrefix is how many leading bytes feed the content hash. Full-file
// hashing would be too slow on multi-GB captures; size + a prefix hash keys the
// cache reliably enough (a re-encode changing only late bytes is not a concern).
const fingerprintPrefix = 1 << 20 // 1 MiB

// Fingerprint returns a stable cache key for a video file: its size plus the
// sha256 of its first MiB.
func Fingerprint(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return "", err
	}
	h := sha256.New()
	if _, err := io.CopyN(h, f, fingerprintPrefix); err != nil && err != io.EOF {
		return "", err
	}
	return fmt.Sprintf("%d-%s", fi.Size(), hex.EncodeToString(h.Sum(nil))[:16]), nil
}

// planArgs returns the ffmpeg arguments to produce out from src. An H.264 source
// is stream-copied into MP4 (exact timeline); any other codec is transcoded to
// H.264/AAC. Neither path forces a frame rate or pre-seeks, so timestamps are
// preserved.
func planArgs(src, out, videoCodec string) []string {
	base := []string{"-nostdin", "-loglevel", "error", "-y", "-i", src}
	if videoCodec == "h264" {
		return append(base, "-c", "copy", "-movflags", "+faststart", out)
	}
	return append(base,
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "20",
		"-c:a", "aac",
		"-movflags", "+faststart", out)
}

// videoCodec probes the first video stream's codec name via ffprobe. On any
// error it returns "" — the caller then transcodes (the safe default).
func videoCodec(path string) string {
	out, err := exec.Command(FFprobeBin, "-v", "error",
		"-select_streams", "v:0", "-show_entries", "stream=codec_name",
		"-of", "csv=p=0", path).Output()
	if err != nil {
		return ""
	}
	return string(bytes.TrimSpace(out))
}

// Build produces (or reuses from cacheDir) an MP4 Playback Proxy of src and
// returns its path plus a non-empty warning when the proxy's duration diverges
// from the source (the Tick may then be untrustworthy).
func Build(src, cacheDir string) (path, warning string, err error) {
	fp, err := Fingerprint(src)
	if err != nil {
		return "", "", err
	}
	out := filepath.Join(cacheDir, fp+".mp4")
	if _, statErr := os.Stat(out); statErr == nil {
		return out, durationWarning(src, out), nil // cache hit
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", "", err
	}

	tmp := out + ".tmp.mp4"
	_ = os.Remove(tmp)
	args := planArgs(src, tmp, videoCodec(src))
	cmd := exec.Command(capture.FFmpegBin, args...)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if runErr := cmd.Run(); runErr != nil {
		_ = os.Remove(tmp)
		return "", "", fmt.Errorf("ffmpeg proxy %s: %w: %s", src, runErr, errb.String())
	}
	if renErr := os.Rename(tmp, out); renErr != nil {
		return "", "", renErr
	}
	return out, durationWarning(src, out), nil
}

// durationWarning returns a message when src and proxy durations differ beyond
// tolerance (max(1s, 2%)), else "". A parity failure means a re-encode shifted
// the timeline and Ticks read off the proxy would be wrong.
func durationWarning(src, proxy string) string {
	sd, err1 := capture.DurationMs(src)
	pd, err2 := capture.DurationMs(proxy)
	if err1 != nil || err2 != nil {
		return ""
	}
	diff := sd - pd
	if diff < 0 {
		diff = -diff
	}
	tol := sd / 50 // 2%
	if tol < 1000 {
		tol = 1000
	}
	if diff > tol {
		return "playback proxy duration differs from the source by " +
			strconv.Itoa(diff) + "ms — video timecodes may be imprecise"
	}
	return ""
}
