// Package archive packs a set of tiles from one pyramid into a single file.
package archive

import (
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"sync"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/codec"
	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/pyramid"
	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

// Address names a tile by zoom, column and row.
type Address struct{ Z, X, Y int }

func (a Address) parent() Address { return Address{a.Z - 1, a.X / 2, a.Y / 2} }

func (a Address) child(k int) Address { return Address{a.Z + 1, 2*a.X + k%2, 2*a.Y + k/2} }

func (a Address) quadrant() int { return a.Y%2*2 + a.X%2 }

func (a Address) String() string { return fmt.Sprintf("%d-%d-%d", a.Z, a.X, a.Y) }

// Mode says what an archive stores.
type Mode byte

const (
	// Chain stores every tile, each against its parent where the parent is in the set.
	Chain Mode = iota
	// Leaves stores only the tiles that their children do not rebuild, and rebuilds the rest.
	Leaves
)

const magic = "UCHA\x02"

var errCorrupt = errors.New("the data is not a tile archive")

func compare(a, b Address) int {
	if a.Z != b.Z {
		return a.Z - b.Z
	}
	if a.Y != b.Y {
		return a.Y - b.Y
	}
	return a.X - b.X
}

func sameRow(a, b Address) bool { return a.Z == b.Z && a.Y == b.Y }

// rebuilds reports whether those children of a tile are in the set and give the tile exactly.
func rebuilds(tiles map[Address]*terrarium.Tile, a Address) bool {
	var children [4]*terrarium.Tile
	for k := range children {
		if children[k] = tiles[a.child(k)]; children[k] == nil {
			return false
		}
	}
	built, err := pyramid.Parent(children, a.Z)
	return err == nil && built.Width == tiles[a].Width && slices.Equal(built.Samples, tiles[a].Samples)
}

// Pack writes the tiles into one archive.
func Pack(tiles map[Address]*terrarium.Tile, mode Mode) ([]byte, error) {
	var stored, derived []Address
	for a := range tiles {
		if mode == Leaves && rebuilds(tiles, a) {
			derived = append(derived, a)
		} else {
			stored = append(stored, a)
		}
	}
	slices.SortFunc(stored, compare)
	slices.SortFunc(derived, compare)
	packed := make([][]byte, len(stored))
	errs := make([]error, len(stored))
	var wg sync.WaitGroup
	slots := make(chan struct{}, runtime.NumCPU())
	// In leaves mode the tiles of one row are coded one after another. Each starts
	// from what the model learned on the tile before it. Rows run side by side.
	for first := 0; first < len(stored); {
		last := first + 1
		for mode == Leaves && last < len(stored) && sameRow(stored[first], stored[last]) {
			last++
		}
		wg.Add(1)
		go func(first, last int) {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			var start *codec.Start
			for i := first; i < last; i++ {
				a := stored[i]
				if mode == Leaves {
					packed[i], start, errs[i] = codec.EncodeFrom(tiles[a], start)
					continue
				}
				var parent *codec.Parent
				if up := tiles[a.parent()]; a.Z > 0 && up != nil {
					parent = &codec.Parent{Tile: up, Quadrant: a.quadrant()}
				}
				packed[i], errs[i] = codec.Encode(tiles[a], parent)
			}
		}(first, last)
		first = last
	}
	wg.Wait()
	out := append([]byte(magic), byte(mode))
	out = binary.AppendUvarint(out, uint64(len(stored)))
	for i, a := range stored {
		if errs[i] != nil {
			return nil, fmt.Errorf("%s: %w", a, errs[i])
		}
		out = appendAddress(out, a)
		out = binary.AppendUvarint(out, uint64(len(packed[i])))
		out = append(out, packed[i]...)
	}
	out = binary.AppendUvarint(out, uint64(len(derived)))
	for _, a := range derived {
		out = appendAddress(out, a)
	}
	return out, nil
}

func appendAddress(out []byte, a Address) []byte {
	out = binary.AppendUvarint(out, uint64(a.Z))
	out = binary.AppendUvarint(out, uint64(a.X))
	return binary.AppendUvarint(out, uint64(a.Y))
}

type reader struct {
	data []byte
	err  error
}

func (r *reader) uvarint() int {
	v, n := binary.Uvarint(r.data)
	if n <= 0 || v > 1<<31 {
		r.err = errCorrupt
		return 0
	}
	r.data = r.data[n:]
	return int(v)
}

func (r *reader) address() Address {
	return Address{Z: r.uvarint(), X: r.uvarint(), Y: r.uvarint()}
}

// Unpack restores every tile of an archive.
func Unpack(data []byte) (map[Address]*terrarium.Tile, error) {
	if len(data) < len(magic)+1 || string(data[:len(magic)]) != magic {
		return nil, errCorrupt
	}
	mode := Mode(data[len(magic)])
	r := &reader{data: data[len(magic)+1:]}
	tiles := map[Address]*terrarium.Tile{}
	var start *codec.Start
	var before Address
	// The stored tiles are in order of zoom, so a parent is restored before its children.
	for n := r.uvarint(); n > 0 && r.err == nil; n-- {
		a := r.address()
		size := r.uvarint()
		if r.err != nil || size > len(r.data) {
			return nil, errCorrupt
		}
		var tile *terrarium.Tile
		var err error
		if mode == Leaves {
			if !sameRow(before, a) {
				start = nil
			}
			tile, start, err = codec.DecodeFrom(r.data[:size], start)
			before = a
		} else {
			var parent *codec.Parent
			if codec.NeedsParent(r.data[:size]) {
				up := tiles[a.parent()]
				if a.Z == 0 || up == nil {
					return nil, fmt.Errorf("%s was coded against a parent that the archive does not hold", a)
				}
				parent = &codec.Parent{Tile: up, Quadrant: a.quadrant()}
			}
			tile, err = codec.Decode(r.data[:size], parent)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", a, err)
		}
		tiles[a], r.data = tile, r.data[size:]
	}
	derived := make([]Address, r.uvarint())
	for i := range derived {
		derived[i] = r.address()
	}
	if r.err != nil {
		return nil, r.err
	}
	// The deepest tiles come first, so the children of a tile are there when it is built.
	slices.SortFunc(derived, func(a, b Address) int { return compare(b, a) })
	for _, a := range derived {
		var children [4]*terrarium.Tile
		for k := range children {
			if children[k] = tiles[a.child(k)]; children[k] == nil {
				return nil, fmt.Errorf("%s is to be rebuilt from a child that the archive does not hold", a)
			}
		}
		tile, err := pyramid.Parent(children, a.Z)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", a, err)
		}
		tiles[a] = tile
	}
	return tiles, nil
}
