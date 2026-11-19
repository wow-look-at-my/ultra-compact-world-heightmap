// fit-grid.mjs <source.raw> <tile.rgb> <x> <y> <step> <passes> <out dir>
// [gdal.f32] Fits a grid on the lattice of Copernicus tile N27. This covers
// e086 to one served tile. It fits twice. The first fit starts from the true
// grid.
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
const [sourcePath, tilePath, xs, ys, stepText, passText, out, gdalPath] = process.argv.slice(2);
const tx = +xs, ty = +ys, step = +stepText, passes = +passText, q = 0.5, side = 512, N = side * side;
mkdirSync(out, { recursive: true });

const floats = path => { const b = readFileSync(path); return new Float32Array(b.buffer, b.byteOffset, b.length / 4); };
const src = floats(sourcePath), sourceWidth = 3600, west = 86, north = 28;
const rgb = readFileSync(tilePath);
const tile = new Float64Array(N);
for (let i = 0; i < N; i++) tile[i] = (rgb[3 * i] << 16 | rgb[3 * i + 1] << 8 | rgb[3 * i + 2]) / 256 - 32768;

const R = 6378137, half = Math.PI * R, span = 2 * half / 4096, res = span / side;
const fx = new Float64Array(side), fy = new Float64Array(side);
for (let c = 0; c < side; c++) fx[c] = ((-half + tx * span + (c + 0.5) * res) / R * 180 / Math.PI - west) * 3600;
for (let r = 0; r < side; r++) fy[r] = (north - Math.atan(Math.sinh((half - ty * span - (r + 0.5) * res) / R)) * 180 / Math.PI) * 3600;
const i0 = Math.floor(fx[0]) - 1, j0 = Math.floor(fy[0]) - 1;
const GW = Math.floor(fx[side - 1]) + 2 - i0 + 1, GH = Math.floor(fy[side - 1]) + 2 - j0 + 1;
const spline = t => [(1 - t) ** 3 / 6, (3 * t ** 3 - 6 * t ** 2 + 4) / 6, (-3 * t ** 3 + 3 * t ** 2 + 3 * t + 1) / 6, t ** 3 / 6];
const wx = [], wy = [], bx = new Int32Array(side), by = new Int32Array(side);
for (let c = 0; c < side; c++) { const f = Math.floor(fx[c]); wx.push(spline(fx[c] - f)); bx[c] = f - 1 - i0; }
for (let r = 0; r < side; r++) { const f = Math.floor(fy[r]); wy.push(spline(fy[r] - f)); by[r] = f - 1 - j0; }

const warp = grid => {
	const w = new Float64Array(N);
	for (let r = 0; r < side; r++) for (let c = 0; c < side; c++) {
		let v = 0;
		for (let b = 0; b < 4; b++) {
			let row = 0;
			for (let a = 0; a < 4; a++) row += wx[c][a] * grid[(by[r] + b) * GW + bx[c] + a];
			v += wy[r][b] * row;
		}
		w[r * side + c] = v;
	}
	return w;
};
const differ = w => { let n = 0; for (let i = 0; i < N; i++) if (Math.round(w[i] / q) * q !== tile[i]) n++; return n; };

const truth = new Float64Array(GW * GH);
for (let j = 0; j < GH; j++) for (let i = 0; i < GW; i++) truth[j * GW + i] = src[(j0 + j) * sourceWidth + i0 + i];
const exact = warp(truth);
let report = `the true grid: ${differ(exact)} samples differ from the tile`;
if (gdalPath) {
	const gdal = floats(gdalPath);
	let worst = 0;
	for (let i = 0; i < N; i++) worst = Math.max(worst, Math.abs(exact[i] - gdal[i]));
	report += `, and this warp is within ${worst.toFixed(5)} m of gdalwarp`;
}
console.log(report);

const touch = Array.from({ length: GW * GH }, () => []);
for (let r = 0; r < side; r++) for (let c = 0; c < side; c++) for (let b = 0; b < 4; b++) for (let a = 0; a < 4; a++) {
	const weight = wx[c][a] * wy[r][b];
	if (weight > 1e-4) touch[(by[r] + b) * GW + bx[c] + a].push(r * side + c, weight);
}

// unwarp gives the transpose of warp: each grid sample gets the weighted sum of its tile samples.
const unwarp = v => {
	const g = new Float64Array(GW * GH);
	for (let r = 0; r < side; r++) for (let c = 0; c < side; c++) {
		const x = v[r * side + c];
		for (let b = 0; b < 4; b++) for (let a = 0; a < 4; a++) g[(by[r] + b) * GW + bx[c] + a] += wx[c][a] * wy[r][b] * x;
	}
	return g;
};

// guess finds a grid from the tile alone. The least-squares grid whose warp is nearest
// to the tile, by conjugate gradients on the normal equations.
const guess = () => {
	let mean = 0;
	for (let i = 0; i < N; i++) mean += tile[i] / N;
	const g = new Float64Array(GW * GH).fill(mean);
	const left = warp(g).map((v, i) => tile[i] - v);
	let s = unwarp(left), p = Float64Array.from(s), gamma = s.reduce((t, v) => t + v * v, 0);
	for (let k = 0; k < 200 && gamma > 1e-12; k++) {
		const ap = warp(p);
		const alpha = gamma / ap.reduce((t, v) => t + v * v, 0);
		for (let i = 0; i < g.length; i++) g[i] += alpha * p[i];
		for (let i = 0; i < N; i++) left[i] -= alpha * ap[i];
		s = unwarp(left);
		const next = s.reduce((t, v) => t + v * v, 0);
		for (let i = 0; i < p.length; i++) p[i] = s[i] + next / gamma * p[i];
		gamma = next;
	}
	return g;
};

const cost = (v, i) => { const d = Math.abs(v - tile[i]) - q / 2 + 0.02; return d > 0 ? d * d : 0; };
const fit = (start, name) => {
	const grid = start.map(v => Math.round(v / step) * step);
	const w = warp(grid);
	const before = differ(w);
	for (let pass = 0; pass < passes; pass++) {
		let moved = 0;
		for (let k = 0; k < grid.length; k++) {
			const t = touch[k];
			let best = 0, bestGain = 0;
			for (const move of [-step, step]) {
				let gain = 0;
				for (let n = 0; n < t.length; n += 2) gain += cost(w[t[n]], t[n]) - cost(w[t[n]] + move * t[n + 1], t[n]);
				if (gain > bestGain + 1e-12) { bestGain = gain; best = move; }
			}
			if (best !== 0) { grid[k] += best; moved++; for (let n = 0; n < t.length; n += 2) w[t[n]] += best * t[n + 1]; }
		}
		if (moved === 0) break;
	}
	console.log(`a grid with a step of ${step} m, from ${name}: ${before} samples differ before the fit, ${differ(w)} after it`);
	mkdirSync(`${out}/${name}`, { recursive: true });
	writeFileSync(`${out}/${name}/grid.f32`, Buffer.from(Float32Array.from(grid).buffer));
	writeFileSync(`${out}/${name}/warp.f32`, Buffer.from(Float32Array.from(warp(grid)).buffer));
	writeFileSync(`${out}/${name}/size.txt`, `${GW} ${GH}\n`);
};
fit(truth, 'truth');
fit(guess(), 'tile');
