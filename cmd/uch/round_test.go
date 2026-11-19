package main

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func heights(values ...float32) []byte {
	out := make([]byte, 0, 4*len(values))
	for _, v := range values {
		out = binary.LittleEndian.AppendUint32(out, math.Float32bits(v))
	}
	return out
}

func TestHeightsRoundHalfToEvenAtTheStepOfTheZoom(t *testing.T) {
	// A later zoom has a step of half a metre, and a later zoom a step of one metre.
	tile, err := roundHeights(heights(0, 0.25, 0.75, -0.25, 100.3, 8848.86), 3, 2, 12)
	require.NoError(t, err)
	assert.Equal(t, []int32{seaSample, seaSample, seaSample + 256, seaSample, seaSample + 100*256 + 128, seaSample + 8849*256}, tile.Samples)
	tile, err = roundHeights(heights(0.5, 1.5, 2.4), 3, 1, 11)
	require.NoError(t, err)
	assert.Equal(t, []int32{seaSample, seaSample + 512, seaSample + 512}, tile.Samples)
}

func TestHeightsThatATileCannotHoldAreRefused(t *testing.T) {
	_, err := roundHeights(heights(-9999), 1, 1, 12)
	require.NoError(t, err)
	_, err = roundHeights(heights(-40000), 1, 1, 12)
	require.ErrorContains(t, err, "outside")
	_, err = roundHeights(heights(float32(math.NaN())), 1, 1, 12)
	require.ErrorContains(t, err, "outside")
	_, err = roundHeights(heights(1, 2), 3, 1, 12)
	require.ErrorContains(t, err, "3x1")
}

func TestRoundWritesATileThatReferenceModeCanUse(t *testing.T) {
	t.Serial("the commands share one root command and its flags")
	dir := t.TempDir()
	want, _, err := loadTile(fixture)
	require.NoError(t, err)
	// The raster is the fixture itself with noise far under half a step, and a few heights moved across a rounding point.
	values := make([]float32, len(want.Samples))
	for i, v := range want.Samples {
		values[i] = float32(float64(v-seaSample)/256) + float32(i%7-3)*0.01
	}
	for _, i := range []int{5, 1000, 70000, 200000} {
		values[i] += 0.5
	}
	raster, ref := filepath.Join(dir, "warp.f32"), filepath.Join(dir, "warp.png")
	require.NoError(t, os.WriteFile(raster, heights(values...), 0o644))
	_, err = run(t, "round", "--zoom", "12", raster, ref)
	require.NoError(t, err)

	packed, restored := filepath.Join(dir, "tile.uch"), filepath.Join(dir, "tile.png")
	out, err := run(t, "encode", "--reference", ref, fixture, packed)
	require.NoError(t, err)
	assert.Contains(t, out, "decodes with its reference")
	info, err := os.Stat(packed)
	require.NoError(t, err)
	assert.Less(t, info.Size(), int64(64))

	_, err = run(t, "decode", packed, restored)
	require.ErrorContains(t, err, "--reference")
	_, err = run(t, "decode", "--reference", ref, packed, restored)
	require.NoError(t, err)
	got, _, err := loadTile(restored)
	require.NoError(t, err)
	assert.Equal(t, want.Samples, got.Samples)

	// The heights themselves are a reference too.
	_, err = run(t, "encode", "--reference", raster, fixture, packed)
	require.NoError(t, err)
	_, err = run(t, "decode", "--reference", raster, packed, restored)
	require.NoError(t, err)
	got, _, err = loadTile(restored)
	require.NoError(t, err)
	assert.Equal(t, want.Samples, got.Samples)
	short := filepath.Join(dir, "short.f32")
	require.NoError(t, os.WriteFile(short, heights(1, 2, 3), 0o644))
	_, err = run(t, "encode", "--reference", short, fixture, packed)
	require.ErrorContains(t, err, "512x512")
	_, err = run(t, "encode", "--reference", filepath.Join(dir, "missing.f32"), fixture, packed)
	require.Error(t, err)
}

func TestReferenceModeFailsOnWhatItCannotUse(t *testing.T) {
	t.Serial("the commands share one root command and its flags")
	dir := t.TempDir()
	packed := filepath.Join(dir, "tile.uch")
	_, err := run(t, "encode", "--reference", filepath.Join(dir, "missing.png"), fixture, packed)
	require.Error(t, err)
	_, err = run(t, "encode", "--reference", fixture, "--parent", fixture, "--quadrant", "0", fixture, packed)
	require.ErrorContains(t, err, "not both")
	_, err = run(t, "encode", "--reference", fixture, fixture, packed)
	require.NoError(t, err)
	_, err = run(t, "decode", "--reference", filepath.Join(dir, "missing.png"), packed, filepath.Join(dir, "out.png"))
	require.Error(t, err)
	_, err = run(t, "round", filepath.Join(dir, "missing.f32"), filepath.Join(dir, "out.png"))
	require.Error(t, err)
	short := filepath.Join(dir, "short.f32")
	require.NoError(t, os.WriteFile(short, heights(1, 2, 3), 0o644))
	_, err = run(t, "round", short, filepath.Join(dir, "out.png"))
	require.ErrorContains(t, err, "512x512")
}
