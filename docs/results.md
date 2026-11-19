# Results

Every number here is a real tile from `https://tiles.mapterhorn.com`, compressed, decompressed again, and compared sample by sample with the WebP it came from. A tile that does not match stops the run, so no size below belongs to a lossy result.

`webp` is the size of the tile as Mapterhorn serves it, which is a lossless WebP.

## How to reproduce

```bash
corpus/fetch.sh tiles
```

```bash
uch bench tiles/test
```

`corpus/` holds tile lists. The codec was tuned on `dev` only. `test` is a held-out set of other places. The `pyramid` lists are complete: every tile under one root, down to the deepest zoom.

## A whole pyramid

This is the result that matters for storing the data set. A pyramid has one tile at its root, and a few times as many at each zoom below.

Ways to store it:

- **alone**: every tile decodes without any other.
- **against a parent**: every tile but the root is coded against the tile above it. Reading a tile means decoding the few tiles above it first.
- **leaves only**: a tile that its children rebuild exactly is not stored. `uch bench` rebuilds each one and compares it before it counts. Reading a tile near the top means decoding every tile under it.

```
Everest, zoom 9 to 12, 85 tiles, global 30 m data

  webp as served               11230100
  each tile alone               5327486   47.44%
  each tile against a parent    3879085   34.54%
  leaves only                   3295474   29.35%    21 tiles rebuilt, all exact

Interlaken, zoom 14 to 17, 85 tiles, 0.5 m data

  webp as served                9108348
  each tile alone               5124154   56.26%
  each tile against a parent    4164383   45.72%
  leaves only                   3677224   40.37%    21 tiles rebuilt, all exact
```

Lossless JPEG XL is the strongest general image format that was measured on the same tiles:

```
Everest, the same 85 tiles

  webp as served                                   11230100
  jpeg xl 0.7.0, -e 9, the Terrarium RGB image      9288204   82.71%
  jpeg xl 0.11.1, -e 9, one grey channel            7295812   64.97%
  jpeg xl 0.11.1, -e 10 -E 3 -I 100, one grey       7278450   64.81%
  each tile alone                                   5327486   47.44%

zoom 12 only, 64 tiles

  webp as served                                    8074280
  jpeg xl 0.11.1, -e 10 -E 3 -I 100, one grey       4891442
  each tile alone                                   3295474
  each tile against a parent                        2449768
```

The grey image gives JPEG XL its best case. Each sample is `(sample - least sample of the tile) >> shift` in one 16-bit channel. The encoder gets the offset and the quantum of the tile for nothing. One tile was decoded again and compared.

`uch pack` writes such an archive, and `uch unpack` restores every tile from it. The Everest set was packed, unpacked, and the unpacked tiles packed again. The encoder is a pure function of the samples, so equal archives mean equal samples:

```
uch pack --mode leaves    85 tiles, 11230100 bytes to 3296099 bytes, 29.35%
                          the unpacked tiles pack to the same 3296099 bytes
uch pack --mode chain     85 tiles, 11230100 bytes to 3879773 bytes, 34.55%
                          the unpacked tiles pack to the same 3879773 bytes
```

An archive is a little larger than the sum above, because it also holds the address of each tile.

## With the source data at hand

The pipeline source says how a deepest tile is made. `gdalwarp -r cubicspline` reprojects the source data. The result is rounded to the step of the zoom. The global source is the Copernicus 30 m data, which is public.

So a reader who holds that data can make almost every sample of a tile. `corpus/source-check.sh` runs the same `gdalwarp` command on the Copernicus tiles under a served tile. It then codes the served tile against the heights that come out, before any rounding, with `uch encode --reference`. It decodes each tile again and compares it.

```
GDAL 3.8.4, sizes in bytes

20 served tiles at zoom 12 near Everest, over Copernicus tile N27 E086

  webp as served               2700306
  each tile alone              1120739   41.50%
  against the local warp          1485    0.055%   68 to 82 bytes for each tile

other places, one tile each at zoom 12                 webp     alone   against the warp

  Everest, over two Copernicus tiles  12/3035/1715   127908     52535       69
  Kilimanjaro                         12/2473/2082   112610     41210       70
  Aconcagua                           12/1251/2441   127556     52017       76
  Kamchatka                           12/3875/1274    88554     27884       67
  Amazon lowland                      12/1365/2082    98004     45278       69
  Sahara                              12/2161/1766    90988     37124       68
  Cape Town, mostly sea               12/2258/2464    10912      4042       16

places where the server has deeper zooms, from a finer source

  Denali                              12/329/1116    115336     54346   240883
  Mont Blanc                          12/2126/1459   172928    117688   205991
```

A tile differs from the local result in a few hundred samples of 262144, each by one step. Those samples lie next to a rounding point. The decoder sees the same heights, so it knows where to look, and the bytes say which of those samples moved.

The rows for Denali and Mont Blanc show where this stops. Where a finer source exists, the tile is not a reprojection of the Copernicus data. It is the mean of deeper tiles. The same idea then applies at the deepest zoom, with the finer source:

```
Interlaken at zoom 17, against the public swissALTI3D tile 2631-1170 of 2025, 0.5 m

                    webp     alone   against the local warp
  17/68394/46267   79568     45967     1834
  17/68395/46267   74974     45664     1519
  17/68394/46268   71358     41905     1519
  17/68395/46268   78994     50152     1571
  all four        304894    183688     6443    2.11% of webp
```

The step of these tiles is much finer, so more heights lie next to a rounding point. The older swissALTI3D tile of 2019 does not match the served tiles at all.

The script takes tile addresses. For a zoom other than the global one, it takes a source of its own:

```bash
corpus/source-check.sh work
```

```bash
corpus/source-check.sh work 3035/1715 2473/2082 1251/2441 3875/1274 1365/2082 2161/1766 2258/2464 329/1116 2126/1459
```

```bash
ZOOM=17 SOURCE=swissalti3d_2025_2631-1170_0.5_2056_5728.tif corpus/source-check.sh work 68394/46267 68395/46267 68394/46268 68395/46268
```

This is not a tile that stands alone. The reader must hold the source data in the version that the server used. The reader must also get the same answer from GDAL as the writer did. A tile that blends sources was not checked.

Each tile above the deepest zoom follows from the tiles below it. So with the source at hand, a whole pyramid costs what its deepest tiles cost.

## Can the source grid replace the tile?

A zoom 12 tile over this ground holds several times as many samples as the source grid under it. A format can store the grid and rebuild the tile from it. The same script measures that. It rounds the source grid to a step, compresses windows of it, and codes the served tiles against the warp of the rounded grid.

```
the ground under the 20 tiles: 5242880 tile samples, 1770521 samples of the source grid

step of the          bits for each    the grid under    the tiles against    both
source grid          grid sample      the tiles         its warp
1 m                      3.33             736911            463608          1200519
0.5 m, as the tile       4.28             946979            238504          1185483
0.25 m                   5.26            1165018            120285          1285303
0.125 m                  6.26            1385840             60504          1446344

the 20 tiles coded directly, each alone                                     1120739
```

The grid sizes come from square windows of the source grid over the same ground. Each is scaled to the count of grid samples under the tiles.

It does not. A coarse grid is cheap, and the tile then differs from its warp in many places. A fine grid leaves few differences, and costs more than the tile. Every row is larger than the tile coded directly. The best row is larger by about a twentieth.

This grid is the true source, rounded. An encoder that has only the tile must find a grid for itself, and can pick the one that fits the tile best. That encoder was not built.

The row at the step of the tile also says where the direct coder stands. The tile costs 5.06 bits for each sample of the source grid under it. The grid alone, at the step of the tile, costs 4.28, and that grid is not enough to make the tile. So the extra samples that the reprojection adds for each grid sample are almost free already.

## Tile by tile, on places the codec was not tuned on

```
test set, 187 tiles

zoom  tiles      webp     alone  of webp | with parent     child  of webp  of alone
  1       2    212686    172617   81.16% |
  2       8    830564    677497   81.57% |      4         263556   62.35%    75.58%
  3      31   2422496   1953928   80.66% |     15         932642   61.01%    75.11%
  4      12   1695762   1435099   84.63% |      5         430277   64.73%    75.54%
  5      13   2066376   1738476   84.13% |     13        1307501   63.28%    75.21%
  6      13   2215214   1826622   82.46% |     13        1368710   61.79%    74.93%
  7      14   2346164   1898408   80.92% |     14        1418987   60.48%    74.75%
  8      14   2120130   1644732   77.58% |     14        1221237   57.60%    74.25%
  9      14   1962156   1473990   75.12% |     14        1084115   55.25%    73.55%
 10      14   1583558   1127901   71.23% |     14         811656   51.26%    71.96%
 11      14   1124518    747962   66.51% |     14         529703   47.10%    70.82%
 12      14   1221180    636506   52.12% |     14         515213   42.19%    80.94%
 13       6    696532    413759   59.40% |      6         346040   49.68%    83.63%
 14       5    569214    352490   61.93% |      5         299950   52.70%    85.09%
 15       5    558532    325940   58.36% |      5         273534   48.97%    83.92%
 16       5    463056    225798   48.76% |      5         187139   40.41%    82.88%
 17       3    196356    104140   53.04% |      3          87862   44.75%    84.37%
all     187  22284494  16755865   75.19% |    158       11078122   56.12%    75.48%
```

The `with parent` columns count only the tiles whose parent is in the set. All of those followed the parent rule, so none fell back to a tile that decodes alone.

The `dev` set, which the codec was tuned on, gives the same picture: 75.18% alone and 56.72% against a parent.

## What the numbers say

- **Deep zooms compress best.** They are smooth, so prediction leaves little. They are also almost all of the data, because each zoom has a few times the tiles of the above.
- **Shallow zooms are rough.** One sample covers kilometres, so neighbours differ by tens of metres and little is predictable. Alone, the gain over WebP is small. Against a parent it is larger, because the parent fixes the sum of every block of multiple samples.
- **The parent rule is exact.** In every family checked, the served parent is the rounded mean of its served children, sample for sample.
- **An all-water tile with a few islands is smaller than the WebP too.** It was larger until the stream asked first whether a sample is at sea level.

```
testdata/tiles/2-0-2.webp    webp 566    alone 358
```

## Cost

```
one tile of 512 by 512 samples, one core, measured with the uch command

encode alone              2.35 s
decode alone              2.40 s
encode against a parent   2.28 s
decode against a parent   2.27 s
```

Decoding costs the same as encoding, because the decoder rebuilds the model that the encoder used. This codec trades time for size on purpose.

## Where it stopped, and why

This is not a proven floor. It is the point where each further idea of the same kind bought less than a hundredth.

```
dev set, the same tiles before and after all model work

                              first version    final      change
197 tiles, each alone           17071364      16670686    -2.3%
173 tiles, against a parent     12123492      11418093    -5.8%

the last round of ideas, each measured alone on 23 parent and child pairs

rehearsing on the parent        -0.7%
sharing tables across a block   -0.3%
the stage after the mixer       -0.5%
larger neighbourhoods           -0.9%  for twice the time
```

Things set the size that is left:

- **Rough tiles hold real information.** At a shallow zoom one sample covers kilometres, and its neighbours do not say much about it. Coding against the parent removes about a quarter, which is one sample in four.
- **Smooth tiles sit near a rounding limit.** Each neighbour is known only to the nearest step. A linear predictor inherits that error, and a few of them together leave about one to a couple of bits for each sample. This is what the deep zooms cost.

The second limit belongs to linear prediction, not to the data. A model that tracked the range of each height before rounding, or a learned predictor, can go under it. Neither was built. `tried.md` says what a cheap version of the first one did.

For the global tiles, the source grid above bounds what such a model can win. A tile coded directly costs little more than its source grid at the same step. The only large step left is to hold the source data itself, which the reference mode does.

## What was not measured

Every run above was on one amd64 machine. The predictors use floating point, and the code rounds each product on its own so that every platform computes the same value. Nobody has yet encoded a tile on one architecture and decoded it on another.
