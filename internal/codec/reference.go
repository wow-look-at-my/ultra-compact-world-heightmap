// reference.go codes a tile as its difference from a tile that is almost equal to it.

package codec

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

const (
	flagReference = flagChild | flagConstant
	// nearClasses sorts the difference of a neighbour: none, one step, more.
	nearClasses = 3
	// rareLimit lets a probability fall far enough for an event that is rare.
	rareLimit = 1023
)

// NeedsReference reports whether a compressed tile decodes only with its reference.
func NeedsReference(data []byte) bool {
	return len(data) > 0 && data[0]&flagReference == flagReference
}

// differences codes one difference for each sample, in row order. It reads diff when it
// encodes and fills diff when it decodes.
type differences struct {
	rc   *residualCoder
	zero [nearClasses * nearClasses * nearClasses]counter
	sign [3]counter
}

func newDifferences(c *coder) *differences {
	d := &differences{rc: &residualCoder{c: c}}
	for i := range d.rc.escape {
		d.rc.escape[i] = counterInit
	}
	for i := range d.zero {
		d.zero[i] = counterInit
	}
	for i := range d.sign {
		d.sign[i] = counterInit
	}
	return d
}

func nearClass(v int32) int {
	if v < 0 {
		v = -v
	}
	return int(min(v, nearClasses-1))
}

func signClass(v int32) int {
	switch {
	case v < 0:
		return 1
	case v > 0:
		return 2
	}
	return 0
}

func (d *differences) run(diff []int32, w, h int) {
	c := d.rc.c
	at := func(x, y int) int32 {
		if x < 0 || y < 0 || x >= w {
			return 0
		}
		return diff[y*w+x]
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			west, north := at(x-1, y), at(x, y-1)
			corners := min(nearClass(at(x-1, y-1))+nearClass(at(x+1, y-1)), nearClasses-1)
			zero := &d.zero[(nearClass(west)*nearClasses+nearClass(north))*nearClasses+corners]
			v := diff[y*w+x]
			changed := c.bit(b2i(v != 0), zero.p())
			zero.update(changed, rareLimit)
			if changed == 0 {
				diff[y*w+x] = 0
				continue
			}
			side := signClass(west)
			if side == 0 {
				side = signClass(north)
			}
			negative := d.rc.plain(&d.sign[side], b2i(v < 0)) == 1
			size := v
			if v < 0 {
				size = -v
			}
			size = int32(d.rc.codeEscape(int(size)-1)) + 1
			if negative {
				size = -size
			}
			diff[y*w+x] = size
		}
	}
}

// reference gives each sample of the reference tile, rounded to units of the quantum.
func reference(ref *terrarium.Tile, shift uint) []int32 {
	out := make([]int32, len(ref.Samples))
	half := int32(1) << shift >> 1
	for i, v := range ref.Samples {
		out[i] = (v + half) >> shift
	}
	return out
}

// EncodeReference compresses a tile as its difference from a tile of the same size.
// The result is small only when the samples of both tiles are almost all equal.
func EncodeReference(t, ref *terrarium.Tile) ([]byte, error) {
	if t.Width < 1 || t.Height < 1 || t.Width > maxSide || t.Height > maxSide || len(t.Samples) != t.Width*t.Height {
		return nil, fmt.Errorf("a tile of %dx%d with %d samples cannot be encoded", t.Width, t.Height, len(t.Samples))
	}
	if ref.Width != t.Width || ref.Height != t.Height || len(ref.Samples) != len(t.Samples) {
		return nil, fmt.Errorf("the reference is %dx%d and the tile is %dx%d", ref.Width, ref.Height, t.Width, t.Height)
	}
	shift := quantumShift(t.Samples)
	base := reference(ref, shift)
	diff := make([]int32, len(t.Samples))
	for i, v := range t.Samples {
		if v < 0 || v >= 1<<24 {
			return nil, fmt.Errorf("sample %d is %d, outside the 24 bits of a tile", i, v)
		}
		diff[i] = v>>shift - base[i]
	}
	head := byte(shift) | flagReference
	if t.Width == standardSize && t.Height == standardSize {
		head |= flagStandardSize
	}
	out := []byte{head}
	if head&flagStandardSize == 0 {
		out = binary.AppendUvarint(out, uint64(t.Width))
		out = binary.AppendUvarint(out, uint64(t.Height))
	}
	c := newEncoder()
	newDifferences(c).run(diff, t.Width, t.Height)
	return append(out, c.finish()...), nil
}

// DecodeReference restores a tile that EncodeReference wrote. The reference must hold
// the samples that the encoder read.
func DecodeReference(data []byte, ref *terrarium.Tile) (*terrarium.Tile, error) {
	if !NeedsReference(data) {
		return nil, errCorrupt
	}
	head := data[0]
	data = data[1:]
	shift := uint(head & 31)
	if shift > 24 {
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
	if ref == nil {
		return nil, errors.New("the tile was coded against a reference, and decodes only with it")
	}
	if ref.Width != w || ref.Height != h || len(ref.Samples) != w*h {
		return nil, fmt.Errorf("the reference is %dx%d and the tile is %dx%d", ref.Width, ref.Height, w, h)
	}
	t := &terrarium.Tile{Width: w, Height: h, Samples: make([]int32, w*h)}
	newDifferences(newDecoder(data)).run(t.Samples, w, h)
	for i, base := range reference(ref, shift) {
		v := base + t.Samples[i]
		if v < 0 || v >= 1<<(24-shift) {
			return nil, fmt.Errorf("%w: sample %d is outside the range of a tile", errCorrupt, i)
		}
		t.Samples[i] = v << shift
	}
	return t, nil
}
