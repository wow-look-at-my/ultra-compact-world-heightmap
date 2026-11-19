# ultra-compact-world-heightmap

A lossless codec for the [Mapterhorn](https://mapterhorn.com/) elevation tiles, and the `uch` CLI that drives it. The tiles are Terrarium-encoded WebP images. The codec stores the same samples in far fewer bytes, by using what a general image format cannot assume about them.

## Layout

| Path | What lives there |
|---|---|
| `internal/codec/` | The format: `Encode`, `Decode`, and everything under them. |
| `internal/codec/format.go` | The header, and the choice between a tile that decodes alone and one coded against its parent. |
| `internal/codec/model.go` | The predictors and the sample loop. One function runs for both directions. |
| `internal/codec/residual.go` | Turns a prediction error into binary decisions, each with a mixed probability. |
| `internal/codec/logistic.go` | The logistic tables, adaptive probabilities, the mixer and the refiner. Integer arithmetic only. |
| `internal/codec/ols.go`, `window.go` | The least-squares predictors. |
| `internal/codec/parent.go` | Lays the parent tile over a tile, and finds the quantum at which the tile follows it. |
| `internal/codec/rangecoder.go` | The binary arithmetic coder. |
| `internal/pyramid/` | The rule that builds a tile from the four tiles below it, as the Mapterhorn pipeline does. |
| `internal/terrarium/` | Reads a tile from WebP or PNG, and writes one as PNG. |
| `cmd/uch/` | Cobra CLI, one command per file, each registering itself in `init()`. |
| `corpus/` | The tile lists behind every published number, and the script that fetches them. |
| `docs/` | `format.md` and `results.md`. |

## What the codec relies on

These facts come from the Mapterhorn pipeline source. Each is checked against served tiles.

- **Heights are rounded to a quantum that depends on the zoom.** A tile holds multiples of a power of two. The codec finds the shift from the samples and codes the quotient.
- **A parent tile is exactly the rounded mean of its children.** Each sample is the mean of a block of multiple samples one zoom below, rounded half to even to the quantum of the parent. `pyramid.Parent` reproduces it bit for bit. `uch bench` rebuilds each tile whose children are at hand, and counts it as rebuilt only when every sample matches.
- **A leaf tile is a smooth resampling of a coarser grid.** The pipeline warps with a cubic spline, so least-squares predictors do well.

## Invariants

- **Lossless, and proven on every run.** `uch bench` decodes each tile it encodes and compares every sample. A size is never reported for a tile that did not round trip.
- **One code path for both directions.** `model.run` and `residualCoder` take the real value when encoding and ignore it when decoding. Do not write a second decoder loop: a difference between the corrupts tiles silently.
- **The decoder must repeat the encoder's arithmetic exactly, on every platform.** The entropy coder uses integers only. The predictors use `float64`, and wrap each product in a `float64(...)` conversion. Go may otherwise fuse a multiply and an add on some architectures, and a fused result differs in the last bit. Keep that pattern in any new floating-point code on the coding path.
- **A child tile needs its parent to decode, and nothing else.** `Encode` falls back to a tile that decodes alone when the samples do not follow the parent. `NeedsParent` tells the two apart.
- **A stream has no end marker and no checksum.** The encoder drops the zero bytes at the end of a stream. The decoder reads zeros past the end. A truncated file therefore decodes to incorrect samples without an error. The container must protect the bytes.
- **`codec.Tune` is for experiments.** The knobs change the model, so a tile encoded with one setting decodes only with the same setting. Nothing a user runs sets them, except `bench --set`.

## Building and testing

Run `go-toolchain` in the repository root. It tidies, vets, formats, runs the tests with a coverage floor, and builds `build/uch`.

The tests use the real tiles in `testdata/tiles/`. The numbers in `docs/results.md` come from `uch bench` over the tiles that `corpus/fetch.sh` downloads. Run both again after any change to the model, and update the document in the same change.
