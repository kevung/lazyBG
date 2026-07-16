// Package dienet executes the learned die-value classifier — the DieNet7
// trained by ml/train_dielabel.py on hand-labeled die-box crops — in pure Go
// (same self-contained rationale as internal/perceive/pointnet; weights in
// the shared LZPN1 format, BatchNorms folded at export by ml/export_die.py).
//
// Architecture (fixed, mirrored from ml/train_dielabel.py DieNet7):
//
//	input  3×48×48 (CHW, [0,1])
//	3 × [conv3x3 pad1 → ReLU → maxpool2]   channels 3→16→32→64
//	global average pool → fc 64→64 → ReLU → fc 64→7
//
// Classes: 0 = junk (not a single readable die), 1..6 = the top-face value.
package dienet

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

// Input geometry and class layout (must match ml/train_dielabel.py).
const (
	In       = 48
	NClasses = 7
)

// Net is a loaded model.
type Net struct {
	convW [3][]float32
	convB [3][]float32
	convC [4]int // 3,16,32,64
	fc0W  []float32
	fc0B  []float32
	fc1W  []float32
	fc1B  []float32
}

// Load reads an LZPN1 weight file written by ml/export_die.py.
func Load(path string) (*Net, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(raw) < 9 || string(raw[:5]) != "LZPN1" {
		return nil, fmt.Errorf("dienet %s: not an LZPN1 weight file", path)
	}
	off := 5
	u32 := func() int {
		v := binary.LittleEndian.Uint32(raw[off:])
		off += 4
		return int(v)
	}
	count := u32()
	tensors := map[string][]float32{}
	shapes := map[string][]int{}
	for t := 0; t < count; t++ {
		if off+4 > len(raw) {
			return nil, fmt.Errorf("dienet %s: truncated", path)
		}
		nameLen := u32()
		name := string(raw[off : off+nameLen])
		off += nameLen
		nd := u32()
		dims := make([]int, nd)
		n := 1
		for i := range dims {
			dims[i] = u32()
			n *= dims[i]
		}
		if off+4*n > len(raw) {
			return nil, fmt.Errorf("dienet %s: tensor %s truncated", path, name)
		}
		data := make([]float32, n)
		for i := range data {
			data[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[off+4*i:]))
		}
		off += 4 * n
		tensors[name] = data
		shapes[name] = dims
	}

	net := &Net{convC: [4]int{3, 16, 32, 64}}
	for k := 0; k < 3; k++ {
		w, ok := tensors[fmt.Sprintf("conv%d.w", k)]
		b, ok2 := tensors[fmt.Sprintf("conv%d.b", k)]
		if !ok || !ok2 {
			return nil, fmt.Errorf("dienet %s: missing conv%d", path, k)
		}
		s := shapes[fmt.Sprintf("conv%d.w", k)]
		if len(s) != 4 || s[0] != net.convC[k+1] || s[1] != net.convC[k] || s[2] != 3 || s[3] != 3 {
			return nil, fmt.Errorf("dienet %s: conv%d shape %v", path, k, s)
		}
		net.convW[k], net.convB[k] = w, b
	}
	net.fc0W, net.fc0B = tensors["fc0.w"], tensors["fc0.b"]
	net.fc1W, net.fc1B = tensors["fc1.w"], tensors["fc1.b"]
	if net.fc0W == nil || net.fc1W == nil || len(net.fc1B) != NClasses {
		return nil, fmt.Errorf("dienet %s: missing or misshaped fc layers", path)
	}
	return net, nil
}

// Forward runs one crop (CHW float32, 3×In×In, [0,1]) and returns 7 logits.
func (n *Net) Forward(x []float32) []float32 {
	h, w := In, In
	for k := 0; k < 3; k++ {
		x = conv3x3ReLU(x, n.convC[k], h, w, n.convC[k+1], n.convW[k], n.convB[k])
		x, h, w = maxPool2(x, n.convC[k+1], h, w)
	}
	c := n.convC[3]
	feat := make([]float32, c)
	area := float32(h * w)
	for ch := 0; ch < c; ch++ {
		var s float32
		for i := ch * h * w; i < (ch+1)*h*w; i++ {
			s += x[i]
		}
		feat[ch] = s / area
	}
	hid := linear(feat, n.fc0W, n.fc0B)
	for i, v := range hid {
		if v < 0 {
			hid[i] = 0
		}
	}
	return linear(hid, n.fc1W, n.fc1B)
}

// The conv/pool/linear primitives mirror internal/perceive/pointnet (kept
// duplicated rather than exported: ~70 lines, two stable consumers).

func conv3x3ReLU(x []float32, inC, h, w, outC int, wt, bias []float32) []float32 {
	out := make([]float32, outC*h*w)
	for oc := 0; oc < outC; oc++ {
		wBase := oc * inC * 9
		for y := 0; y < h; y++ {
			for xx := 0; xx < w; xx++ {
				acc := bias[oc]
				for ic := 0; ic < inC; ic++ {
					kBase := wBase + ic*9
					iBase := ic * h * w
					for ky := -1; ky <= 1; ky++ {
						sy := y + ky
						if sy < 0 || sy >= h {
							continue
						}
						row := iBase + sy*w
						kRow := kBase + (ky+1)*3
						for kx := -1; kx <= 1; kx++ {
							sx := xx + kx
							if sx < 0 || sx >= w {
								continue
							}
							acc += wt[kRow+kx+1] * x[row+sx]
						}
					}
				}
				if acc < 0 {
					acc = 0
				}
				out[oc*h*w+y*w+xx] = acc
			}
		}
	}
	return out
}

func maxPool2(x []float32, c, h, w int) ([]float32, int, int) {
	oh, ow := h/2, w/2
	out := make([]float32, c*oh*ow)
	for ch := 0; ch < c; ch++ {
		for y := 0; y < oh; y++ {
			for xx := 0; xx < ow; xx++ {
				i := ch*h*w + 2*y*w + 2*xx
				m := x[i]
				if v := x[i+1]; v > m {
					m = v
				}
				if v := x[i+w]; v > m {
					m = v
				}
				if v := x[i+w+1]; v > m {
					m = v
				}
				out[ch*oh*ow+y*ow+xx] = m
			}
		}
	}
	return out, oh, ow
}

func linear(x, wt, b []float32) []float32 {
	out := make([]float32, len(b))
	for o := range b {
		acc := b[o]
		row := o * len(x)
		for i, v := range x {
			acc += wt[row+i] * v
		}
		out[o] = acc
	}
	return out
}
