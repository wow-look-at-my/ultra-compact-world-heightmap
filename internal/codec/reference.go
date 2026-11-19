// reference.go codes a tile as its difference from a tile that is almost equal to it.

package codec

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

const (
	flagReference = flagChild | flagConstant
	// nearClasses sorts the difference of a neighbour: none, one step, more.
	nearClasses = 3
	// rareLimit lets a probability fall far enough for an event that is rare.
	rareLimit = 1023
	// gapClasses sorts how near a reference value lies to a rounding point.
	gapClasses = 10
)

// A Reference holds the values that a tile is coded against, in units of one
// sample.
type Reference struct {
	Width, Height int
	Values        []float64
}

// ReferenceTile makes a reference of a tile.
func ReferenceTile(t *terrarium.Tile) *Reference {
	ref := &Reference{Width: t.Width, Height: t.Height, Values: make([]float64, len(t.Samples))}
	for i, v := range t.Samples {
		ref.Values[i] = float64(v)
	}
	return ref
}

// ReferenceHeights makes a reference of heights in metres, in row order.
func ReferenceHeights(metres []float32, width, height int) *Reference {
	ref := &Reference{Width: width, Height: height, Values: make([]float64, len(metres))}
	for i, v := range metres {
		ref.Values[i] = float64((float64(v) + 32768) * 256)
	}
	return ref
}

// rounded gives each value in units of the quantum, rounded half to even as the pipeline
// rounds. The class of a value holds how near it lies to a rounding point, and on which
// side of its rounded value it lies.
func (ref *Reference) rounded(shift uint) (base []int32, class []uint8) {
	base, class = make([]int32, len(ref.Values)), make([]uint8, len(ref.Values))
	quantum := float64(int64(1) << shift)
	for i, v := range ref.Values {
		r := v / quantum
		if !(r > -(1<<30) && r < 1<<30) {
			continue
		}
		whole := math.RoundToEven(r)
		gap := 0.5 - math.Abs(r-whole)
		near := 0
		for near < gapClasses-1 && gap < 0.25 {
			gap *= 2
			near++
		}
		side := 0
		if r < whole {
			side = 1
		} else if r > whole {
			side = 2
		}
		base[i], class[i] = int32(whole), uint8(near*3+side)
	}
	return base, class
}

// NeedsReference reports whether a compressed tile decodes only with its reference.
func NeedsReference(data []byte) bool {
	return len(data) > 0 && data[0]&flagReference == flagReference
}

// differences codes one difference for each sample, in row order. It reads diff when it
// encodes and fills diff when it decodes.
type differences struct {
	rc   *residualCoder
	zero [gapClasses * nearClasses * nearClasses * nearClasses]counter
	sign [9]counter
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

func (d *differences) run(diff []int32, class []uint8, w, h int) {
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
			near, side := int(class[y*w+x])/3, int(class[y*w+x])%3
			zero := &d.zero[((near*nearClasses+nearClass(west))*nearClasses+nearClass(north))*nearClasses+corners]
			v := diff[y*w+x]
			changed := c.bit(b2i(v != 0), zero.p())
			zero.update(changed, rareLimit)
			if changed == 0 {
				diff[y*w+x] = 0
				continue
			}
			beside := signClass(west)
			if beside == 0 {
				beside = signClass(north)
			}
			negative := d.rc.plain(&d.sign[side*3+beside], b2i(v < 0)) == 1
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

// fits reports whether a reference has the size of a tile.
func (ref *Reference) fits(w, h int) error {
	if ref.Width != w || ref.Height != h || len(ref.Values) != w*h {
		return fmt.Errorf("the reference is %dx%d with %d values and the tile is %dx%d", ref.Width, ref.Height, len(ref.Values), w, h)
	}
	return nil
}

// EncodeReference compresses a tile as its difference from a reference of the same size.
// The result is small only when the tile is almost equal to the rounded reference.
func EncodeReference(t *terrarium.Tile, ref *Reference) ([]byte, error) {
	if t.Width < 1 || t.Height < 1 || t.Width > maxSide || t.Height > maxSide || len(t.Samples) != t.Width*t.Height {
		return nil, fmt.Errorf("a tile of %dx%d with %d samples cannot be encoded", t.Width, t.Height, len(t.Samples))
	}
	if err := ref.fits(t.Width, t.Height); err != nil {
		return nil, err
	}
	shift := quantumShift(t.Samples)
	base, class := ref.rounded(shift)
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
	newDifferences(c).run(diff, class, t.Width, t.Height)
	return append(out, c.finish()...), nil
}

// referenceHeader reads the quantum and the size of a tile coded against a reference,
// and gives the stream that follows them.
func referenceHeader(data []byte) (shift uint, w, h int, rest []byte, err error) {
	if !NeedsReference(data) {
		return 0, 0, 0, nil, errCorrupt
	}
	head := data[0]
	data = data[1:]
	if shift = uint(head & 31); shift > 24 {
		return 0, 0, 0, nil, errCorrupt
	}
	w, h = standardSize, standardSize
	if head&flagStandardSize == 0 {
		var sides [2]uint64
		for i := range sides {
			v, n := binary.Uvarint(data)
			if n <= 0 || v < 1 || v > maxSide {
				return 0, 0, 0, nil, errCorrupt
			}
			sides[i], data = v, data[n:]
		}
		w, h = int(sides[0]), int(sides[1])
	}
	return shift, w, h, data, nil
}

// ReferenceSize gives the width and the height of a tile coded against a reference.
func ReferenceSize(data []byte) (width, height int, err error) {
	_, w, h, _, err := referenceHeader(data)
	return w, h, err
}

// DecodeReference restores a tile that EncodeReference wrote. The reference must hold
// the values that the encoder read.
func DecodeReference(data []byte, ref *Reference) (*terrarium.Tile, error) {
	shift, w, h, data, err := referenceHeader(data)
	if err != nil {
		return nil, err
	}
	if ref == nil {
		return nil, errors.New("the tile was coded against a reference, and decodes only with it")
	}
	if err := ref.fits(w, h); err != nil {
		return nil, err
	}
	t := &terrarium.Tile{Width: w, Height: h, Samples: make([]int32, w*h)}
	base, class := ref.rounded(shift)
	newDifferences(newDecoder(data)).run(t.Samples, class, w, h)
	for i, b := range base {
		v := b + t.Samples[i]
		if v < 0 || v >= 1<<(24-shift) {
			return nil, fmt.Errorf("%w: sample %d is outside the range of a tile", errCorrupt, i)
		}
		t.Samples[i] = v << shift
	}
	return t, nil
}
