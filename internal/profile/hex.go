package profile

import (
	"fmt"
	"image/color"
	"strconv"
)

// ParseHex parses a CSS-style hex color ("#rrggbb" or "#rgb", leading '#'
// optional) into an opaque RGBA. It is how the manifest's declared checker
// colors (Session Priors) become a CaptureProfile.
func ParseHex(s string) (color.RGBA, error) {
	if len(s) > 0 && s[0] == '#' {
		s = s[1:]
	}
	var r, g, b uint64
	var err error
	switch len(s) {
	case 6:
		r, err = strconv.ParseUint(s[0:2], 16, 8)
		if err == nil {
			g, err = strconv.ParseUint(s[2:4], 16, 8)
		}
		if err == nil {
			b, err = strconv.ParseUint(s[4:6], 16, 8)
		}
	case 3:
		r, err = strconv.ParseUint(s[0:1]+s[0:1], 16, 8)
		if err == nil {
			g, err = strconv.ParseUint(s[1:2]+s[1:2], 16, 8)
		}
		if err == nil {
			b, err = strconv.ParseUint(s[2:3]+s[2:3], 16, 8)
		}
	default:
		return color.RGBA{}, fmt.Errorf("hex color %q: want #rrggbb or #rgb", s)
	}
	if err != nil {
		return color.RGBA{}, fmt.Errorf("hex color %q: %w", s, err)
	}
	return color.RGBA{uint8(r), uint8(g), uint8(b), 0xff}, nil
}
