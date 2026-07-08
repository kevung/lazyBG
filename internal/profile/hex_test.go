package profile

import (
	"image/color"
	"testing"
)

func TestParseHex(t *testing.T) {
	cases := []struct {
		in   string
		want color.RGBA
	}{
		{"#e1ded2", color.RGBA{0xe1, 0xde, 0xd2, 0xff}},
		{"#464850", color.RGBA{0x46, 0x48, 0x50, 0xff}},
		{"e1ded2", color.RGBA{0xe1, 0xde, 0xd2, 0xff}}, // leading # optional
		{"#FFF", color.RGBA{0xff, 0xff, 0xff, 0xff}},   // short form
	}
	for _, c := range cases {
		got, err := ParseHex(c.in)
		if err != nil {
			t.Errorf("ParseHex(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseHex(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseHex_Rejects(t *testing.T) {
	for _, in := range []string{"", "#12345", "#gggggg", "#12345678"} {
		if _, err := ParseHex(in); err == nil {
			t.Errorf("ParseHex(%q): expected error", in)
		}
	}
}
