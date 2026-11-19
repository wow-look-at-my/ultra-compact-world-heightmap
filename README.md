# ultra-compact-world-heightmap

A lossless codec for the [Mapterhorn](https://mapterhorn.com/) terrain tiles. It stores the exact samples of a Terrarium WebP tile in a fraction of the bytes.

A general image format has to work for any picture. This only has to work for the ground, as one pipeline renders it:

- Heights are smooth, so a predictor that learns the local shape leaves a small error.
- Heights are rounded to a step that depends on the zoom, so the low bits carry nothing.
- Every tile above the deepest zoom is the rounded mean of the tiles below it. A tile can therefore be coded against its parent, or left out and rebuilt from its children.

## Results

Complete pyramids of served tiles, every tile decoded again and compared sample by sample:

```
                               Everest, zoom 9 to 12    Interlaken, zoom 14 to 17
                               global 30 m data         0.5 m data
webp as served                 11230100  100.0%          9108348  100.0%
each tile alone                 5327486   47.4%          5124154   56.3%
each tile against its parent    3879085   34.5%          4164383   45.7%
leaves only, parents rebuilt    3295474   29.4%          3677224   40.4%
```

A reader who holds the public source data can go much further. The deepest global tiles are that data, reprojected and rounded. A tile coded against a local run of the same step is under a hundred bytes:

```
20 served tiles at zoom 12, Everest
webp as served                  2700306  100.0%
each tile alone                 1120739   41.5%
against the local reprojection     1485    0.055%
```

The same holds at the deepest zoom of a finer source: Interlaken tiles against the public swissALTI3D data take 2.1% of their WebP size.

Such a tile does not stand alone. Decoding needs the same source data. For the global data that is all it needs: `uch warp` does the reprojection itself, with the same bits on every machine.

[docs/results.md](docs/results.md) has the tables for each zoom and how to reproduce them. [docs/tried.md](docs/tried.md) lists what worked and what did not.

## Use

```bash
uch encode tile.webp tile.uch
```

```bash
uch decode tile.uch tile.png
```

Code a tile against the tile one zoom above it. Decoding then needs the same parent:

```bash
uch encode --parent 11-1072-717.webp 12-2144-1434.webp 12-2144-1434.uch
```

```bash
uch decode --parent 11-1072-717.webp 12-2144-1434.uch 12-2144-1434.png
```

Code a tile as its difference from a tile that is almost equal to it, such as an older version. Decoding needs the same reference:

```bash
uch encode --reference old.webp new.webp new.uch
```

```bash
uch decode --reference old.webp new.uch new.png
```

A reference can also be heights before rounding, as a `.f32` file of float32 metres. `uch warp` makes those from the Copernicus grid under a tile, the way the pipeline does, and `corpus/source-check.sh` shows every step:

```bash
uch warp --x 3034 --y 1717 --west 86 --north 28 N27E086.f32 12-3034-1717.f32
```

```bash
uch encode --reference 12-3034-1717.f32 12-3034-1717.webp 12-3034-1717.uch
```

Pack every tile of a pyramid into one file. The default stores only the tiles that their children do not rebuild exactly:

```bash
uch pack tiles/ tiles.ucha
```

```bash
uch unpack tiles.ucha restored/
```

Measure a directory of tiles named `z-x-y.webp`. Every tile is decoded again and compared before its size counts:

```bash
uch bench tiles/
```

## Build

```bash
go-toolchain
```

The binary lands in `build/uch`.

## More

- [docs/format.md](docs/format.md) describes the format.
- [AGENTS.md](AGENTS.md) describes the code.

The test tiles in `testdata/tiles/` are © [Mapterhorn](https://mapterhorn.com/attribution) and its data sources.
