# What was tried

Each entry was measured on real tiles with `uch bench`. The sizes are in `results.md`. This page records why the codec looks the way it does, and what did not work, so that nobody tries it twice.

## Kept

- **Least-squares predictors.** They learn the local shape of the ground from the samples already coded. Without them a tile is about a twentieth larger.
- **A fit over the predictions.** A second least-squares fit reads every prediction and corrects their weighted mean. This was the largest single gain after the first version, and most of it is on smooth tiles.
- **Fits that train on a window.** A fit that trains on the rows above and the samples. This is beside the sample helps once the second fit is there to combine it with the others.
- **The parent bound.** The last sample of each block has only a few possible values.
- **The sea level question.** An all-water tile with a few islands was larger than the WebP until the stream asked this first.
- **A reference tile.** A deepest global tile is the public source data, put through the reprojection step of the pipeline, and rounded. A reader who holds that data and runs that step gets all but a few hundred samples of the tile. The tile then costs the bytes that name those samples.
- **Softer weights and a faster mixer.** Small gains each.
- **A stage that corrects the mixed probability.** A small gain.

## Dropped

- **The grid of the source data.** The deepest global tiles are a cubic spline through a coarser grid. The tile address gives the place of each sample in a cell of that grid. The bits per sample were almost flat across that place, so a predictor for each place has nothing to find.
- **Coding the source grid in place of the tile.** A deepest global tile holds samples for each sample of the source grid, so the grid looks cheaper to store. It is not. Rounded to the step of the tile, the grid costs a little less than the tile, and the tile no longer follows from it. With the differences added, or with a finer step, the total is larger than the tile coded directly. `results.md` has the numbers.
- **Smooth surfaces through the parent.** A polynomial fitted to the parent on every side, and a fit of how far each sample lies from it. Alone it made tiles larger. With the second fit it gave a small gain for a few times the work.
- **Soft neighbour values.** Each coded sample kept a guess at its height before rounding: the prediction, pulled into the half step around the sample. Predictors that read those guesses changed nothing.
- **A bias for each context.** The mean error in each context, added back to the prediction. Tiles got larger.
- **The slope of the parent as a context.** No change.
- **A fast twin of each probability table.** Slightly larger.
- **More contexts for the low bits of a large error.** Slightly larger.
- **Slower or faster probability counters.** No change.
- **A guard on the second fit.** A few islands in open water throw a least-squares fit far off. A guard that fell back to the weighted mean where the fit did worse nearby fixed that tile. Cost every other tile more than it saved. The sea level question fixed the tile without it.

## Not tried

- **A learned predictor.** A small network trained on many tiles can find shapes that a linear fit cannot, such as ridges and valleys. It needs exact integer inference to stay lossless on every platform.
- **Exact inference of the height before rounding.** A coded sample says that the true height lies within half a step of it. A model that kept those bounds, and not a point estimate, was not built. The soft values above were a cheap stand-in for this, and did not work. The measurement of the source grid in `results.md` limits what it can give.
