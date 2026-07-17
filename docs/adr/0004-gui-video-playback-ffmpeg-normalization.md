---
status: accepted
---

# GUI video playback: bundled-ffmpeg normalization, served by a local media server; Tick stays on the original timeline

ADR-0002 locked Wails v2 + the OS webview's HTML5 `<video>` on the premise that "scrubbing is
trivial." Building the walking skeleton exposed two problems that premise hid:

1. **Nothing displays in `wails dev`.** The video is wired as `<video src="/media/current">`, served
   by a custom `assetserver.Handler` (`gui/main.go`). In dev, Wails delegates the asset pipeline to
   the Vite dev server, whose SPA fallback returns `index.html` (HTTP 200) for *any* unmatched path —
   including `/media/current`. Wails therefore considers the asset "found" and **never calls the Go
   handler**; the `<video>` receives an HTML document and fails to decode. Confirmed empirically:
   `curl` of `/media/current` on both the Vite port and the Wails DevWebServer returns
   `Content-Type: text/html`. It is a **dev-only** routing bug (a `wails build` has no Vite), but the
   team develops with `make dev`, so it is blocking.
2. **Native `<video>` codec support is not portable.** Decode depends on each end-user's webview and
   plugins — WebView2 (Windows), WKWebView (macOS), WebKitGTK + system GStreamer (Linux). No single
   format is natively playable everywhere without a system dependency. This conflicts with the locked
   decisions on heterogeneous-capture robustness (CLAUDE.md §3.7) and bundled-ffmpeg decode (§3.10).

The system's join key is the **Tick** (a timecode on the original Capture); every observation,
Move Decision, review item, and `.mat` export references it, and the automatic pipeline decodes
perception frames by seeking the **original** file to a `tickMs`. Any playback change must keep the
Tick the user reads/stamps in the GUI identical to that coordinate.

## Decision

- **Serve media from a dedicated local HTTP server** (fixed loopback port, absolute
  `http://127.0.0.1:<port>/media/...` URL, Range via `http.ServeFile`), independent of the
  Wails/Vite asset pipeline. Fixes the dev bug and behaves identically in dev and production.
- **Normalize on open, optimistically** (see the **Playback Proxy** domain term): serve the
  **original** first; on a `<video>` `error`/no-metadata event, build a Proxy with the bundled
  ffmpeg — **remux (`-c copy`) when possible, else transcode to H.264/AAC MP4** — cached by the
  Capture's content hash. Most competition footage (H.264/MP4) needs no Proxy.
- **Canonical output = H.264/MP4.** Guaranteeing WebKitGTK can play it on end-user Linux machines
  without a system `gst-libav` (bundling the GStreamer decoder plugin, or a per-platform output
  format) is a **packaging** concern, tracked separately.
- **Tick stays defined on the original Capture timeline.** The Proxy must preserve that timeline
  (remux is exact; re-encode is fallback-only, no fps resample / no trim); perception always
  decodes the original; duration parity is asserted on open.
- **Stack stays Go + Wails.** Rust/Tauri was considered and rejected (below).

## Considered options

- **Keep native `<video>` on the raw file, just fix the dev route** — rejected: fixes symptom (1)
  but not (2); format support would still vary per end-user webview.
- **Server-side frame streaming (ffmpeg → images/MJPEG, custom player)** — rejected: total format
  control and exact Ticks, but reinvents a video player (audio, smooth playback, speed, seek) at a
  CPU cost that fights the CPU-only / modest-PC constraint (§3.6).
- **Define the Tick on the Proxy timeline / decode perception from the Proxy** — rejected: the Proxy
  is lossy (transcode artifacts could degrade the board reader) and its timeline would not match the
  training crops cut from the original by `lazybg align`.
- **Switch to Rust/Tauri to escape codec limits** — rejected: Tauri uses the *same* OS webviews as
  Wails (WebKitGTK/WebView2/WKWebView) → identical codec limits, zero gain; and it would sacrifice
  the Go gnubg engine (§3.1, the one deliberately-kept asset) and reverse ADR-0002/0003. The lever
  that controls codec portability is the decode/render architecture, not the language.

## Consequences

- Ships a bundled `ffmpeg`/`ffprobe` per platform (already anticipated by §3.10) plus a Proxy cache
  directory (gitignored, cleanable).
- Native `<video>` still provides the primitives (`currentTime`, `duration`, `playbackRate`) the
  custom control bar is built on — this ADR does not reverse ADR-0002, it feeds `<video>` a
  normalized, locally-served source.
- Packaging must make the canonical format self-contained on Linux (bundle the GStreamer H.264
  plugin, or emit a per-platform format) — tracked as a separate packaging ticket.
- `gui/main.go`'s `mediaHandler` moves onto the standalone media server; `OpenResult.VideoURL`
  becomes an absolute loopback URL.
