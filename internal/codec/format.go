// Package codec compresses elevation tiles without loss.
package codec

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

const (
	flagStandardSize = 1 << 5
	flagConstant     = 1 << 6
	standardSize     = 512
	maxSide          = 1 << 14
	seaLevel         = 32768 * 256
)

var errCorrupt = errors.New("the data is not a compressed tile")

// quantumShift counts the low bits that are clear in every sample.
func quantumShift(samples []int32) uint {
	all := int32(0)
	for _, v := range samples {
		all |= v
	}
	shift := uint(0)
	for shift < 24 && all>>shift&1 == 0 {
		shift++
	}
	return shift
}

// Encode compresses a tile.
func Encode(t *terrarium.Tile) ([]byte, error) {
	out, _, err := encode(t, false)
	return out, err
}

// CostMap compresses a tile and reports the bits that each sample took.
func CostMap(t *terrarium.Tile) ([]float32, error) {
	_, costs, err := encode(t, true)
	return costs, err
}

func encode(t *terrarium.Tile, probe bool) ([]byte, []float32, error) {
	if t.Width < 1 || t.Height < 1 || t.Width > maxSide || t.Height > maxSide || len(t.Samples) != t.Width*t.Height {
		return nil, nil, fmt.Errorf("a tile of %dx%d with %d samples cannot be encoded", t.Width, t.Height, len(t.Samples))
	}
	shift := quantumShift(t.Samples)
	q := make([]int32, len(t.Samples))
	constant := true
	for i, v := range t.Samples {
		if v < 0 || v >= 1<<24 {
			return nil, nil, fmt.Errorf("sample %d is %d, outside the 24 bits of a tile", i, v)
		}
		q[i] = v >> shift
		constant = constant && q[i] == q[0]
	}
	head := byte(shift)
	if t.Width == standardSize && t.Height == standardSize {
		head |= flagStandardSize
	}
	if constant {
		head |= flagConstant
	}
	out := []byte{head}
	if head&flagStandardSize == 0 {
		out = binary.AppendUvarint(out, uint64(t.Width))
		out = binary.AppendUvarint(out, uint64(t.Height))
	}
	out = binary.AppendVarint(out, int64(q[0]-seaLevel>>shift))
	costs := make([]float32, len(q))
	if constant {
		return out, costs, nil
	}
	c := newEncoder()
	m := newModel(c, t.Width, t.Height, q[0])
	m.costs, m.rc.probing = costs, probe
	m.run(q)
	return append(out, c.finish()...), costs, nil
}

// Decode restores a tile that Encode wrote.
func Decode(data []byte) (*terrarium.Tile, error) {
	if len(data) < 2 {
		return nil, errCorrupt
	}
	head := data[0]
	data = data[1:]
	shift := uint(head & 31)
	if shift > 24 || head&0x80 != 0 {
		return nil, errCorrupt
	}
	w, h := standardSize, standardSize
	if head&flagStandardSize == 0 {
		var sides [2]uint64
		for i := range sides {
			v, n := binary.Uvarint(data)
			if n <= 0 || v < 1 || v > maxSide {
				return nil, errCorrupt
			}
			sides[i], data = v, data[n:]
		}
		w, h = int(sides[0]), int(sides[1])
	}
	delta, n := binary.Varint(data)
	if n <= 0 {
		return nil, errCorrupt
	}
	data = data[n:]
	base := delta + seaLevel>>shift
	if base < 0 || base >= 1<<(24-shift) {
		return nil, errCorrupt
	}
	t := &terrarium.Tile{Width: w, Height: h, Samples: make([]int32, w*h)}
	if head&flagConstant != 0 {
		for i := range t.Samples {
			t.Samples[i] = int32(base) << shift
		}
		return t, nil
	}
	c := newDecoder(data)
	m := newModel(c, w, h, int32(base))
	m.run(nil)
	if c.overrun() {
		return nil, errCorrupt
	}
	for y := 0; y < h; y++ {
		row := m.img[(y+margin)*m.stride+margin:]
		for x := 0; x < w; x++ {
			v := row[x]
			if v < 0 || v >= 1<<(24-shift) {
				return nil, errCorrupt
			}
			t.Samples[y*w+x] = v << shift
		}
	}
	return t, nil
}
