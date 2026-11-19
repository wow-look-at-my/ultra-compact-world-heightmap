package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/pyramid"
	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

// writeFamily writes rough tiles and the tile that the pipeline builds above them.
func writeFamily(t *testing.T, dir string) {
	t.Helper()
	rng := rand.New(rand.NewSource(3))
	var children [4]*terrarium.Tile
	for k := range children {
		c := &terrarium.Tile{Width: 64, Height: 64, Samples: make([]int32, 64*64)}
		for i := range c.Samples {
			c.Samples[i] = 32768*256 + int32((i%64)*(i/64)/8+rng.Intn(5))<<8
		}
		children[k] = c
		writePNG(t, filepath.Join(dir, fmt.Sprintf("5-%d-%d.png", 6+k%2, 8+k/2)), c)
	}
	parent, err := pyramid.Parent(children, 4)
	require.NoError(t, err)
	writePNG(t, filepath.Join(dir, "4-3-4.png"), parent)
}

func TestPackAndUnpackGiveEveryTileBack(t *testing.T) {
	t.Serial("the commands share one root command and its flags")
	in := t.TempDir()
	writeFamily(t, in)
	for _, mode := range []string{"leaves", "chain"} {
		packed, out := filepath.Join(t.TempDir(), "tiles.ucha"), t.TempDir()
		report, err := run(t, "pack", "--mode", mode, in, packed)
		require.NoError(t, err)
		assert.Contains(t, report, "5 tiles")
		_, err = run(t, "unpack", packed, out)
		require.NoError(t, err)
		for _, name := range []string{"4-3-4", "5-6-8", "5-7-8", "5-6-9", "5-7-9"} {
			want, _, err := loadTile(filepath.Join(in, name+".png"))
			require.NoError(t, err)
			got, _, err := loadTile(filepath.Join(out, name+".png"))
			require.NoError(t, err)
			assert.Equal(t, want.Samples, got.Samples, "%s in %s mode", name, mode)
		}
	}
}

func TestPackAndUnpackFailOnWhatTheyCannotUse(t *testing.T) {
	t.Serial("the commands share one root command and its flags")
	in := t.TempDir()
	writeFamily(t, in)
	_, err := run(t, "pack", "--mode", "everything", in, filepath.Join(t.TempDir(), "tiles.ucha"))
	require.Error(t, err)
	_, err = run(t, "pack", t.TempDir(), filepath.Join(t.TempDir(), "tiles.ucha"))
	require.Error(t, err)
	_, err = run(t, "unpack", "no-such-archive.ucha", t.TempDir())
	require.Error(t, err)
	bad := filepath.Join(t.TempDir(), "bad.ucha")
	require.NoError(t, os.WriteFile(bad, []byte("not an archive"), 0o644))
	_, err = run(t, "unpack", bad, t.TempDir())
	require.Error(t, err)
}
