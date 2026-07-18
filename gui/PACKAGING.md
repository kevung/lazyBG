# Packaging notes

## Linux: self-contained H.264 video playback (ADR-0004)

On Linux the app renders in **WebKitGTK**, which decodes the `<video>` element
through the system's **GStreamer** plugins. H.264 (the canonical Playback-Proxy
format) needs **`gst-libav`**, which is *not* installed on every machine. To
stay self-contained, a release bundle ships the needed plugins next to the
executable and the app points GStreamer at them.

### Runtime hook (implemented)

At startup `useBundledGStreamer()` (`gui/main.go`) looks for a
`gstreamer-1.0/` directory **beside the executable**. If present, it prepends it
to `GST_PLUGIN_PATH_1_0` so bundled decoders win over (or supply what's missing
from) the system. If absent — in dev, or on macOS/Windows — it is a **no-op**
and nothing changes. The path logic lives in `internal/gstbundle` and is
unit-tested.

Expected release layout:

```
lazybg-gui                 # the executable
gstreamer-1.0/             # bundled plugins (this dir triggers the hook)
  libgstlibav.so
  libgstcoreelements.so
  ... + transitive shared-library deps
```

### Bundling the plugins (release step — TODO, needs validation)

Collecting `gst-libav` and its transitive `.so` dependencies into the bundle is
a per-distro packaging step, not done by `make build`. Sketch:

1. Locate the plugin: `gst-inspect-1.0 --plugin avdec_h264` → its `.so` path
   (e.g. `/usr/lib/gstreamer-1.0/libgstlibav.so`).
2. Copy it plus `libgstcoreelements.so` (typefind/decodebin wiring) into
   `gstreamer-1.0/`.
3. Copy the transitive shared-library deps (`ldd` on each plugin →
   `libav*.so*`, etc.) into a bundled `lib/` and set `rpath`/`LD_LIBRARY_PATH`
   accordingly.
4. Run `gst-inspect-1.0` against the bundle on a **clean machine without
   `gst-libav`** to confirm `avdec_h264` resolves.

> **Status:** the runtime hook is implemented and safe (no-op without a bundle).
> The plugin-collection step and end-to-end validation on a codec-less Linux box
> are **not yet done** — they require a clean-room package build. Until then,
> Linux users without `gst-libav` should install it (`gstreamer-plugins-libav`
> or distro equivalent) as a documented prerequisite.

## macOS / Windows

WKWebView (macOS) and WebView2 (Windows) decode H.264/MP4 natively via the OS —
no bundled codec required.
