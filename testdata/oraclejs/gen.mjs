#!/usr/bin/env node
/**
 * Build standard data from scene scripts.
 *
 *   node gen.mjs --from-scenes
 *
 * For each scene:
 *  1) render with CanvasKit
 *  2) if tmp/scenes_gpui/<id>.png exists and is close enough → keep CanvasKit as standard
 *  3) else copy gpui scene PNG if present (oracle=gpui-scene), else CanvasKit
 */
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import { createRequire } from 'module';
import { renderSceneToPNG } from './render_scene.mjs';

const require = createRequire(import.meta.url);
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, '../..');
const catalogPath = path.join(repoRoot, 'testdata/refs/standard.json');
const imagesDir = path.join(repoRoot, 'testdata/refs/standard');
const scenesDir = path.join(repoRoot, 'testdata/scenes');
const ckDir = path.join(repoRoot, 'tmp', 'scenes_canvaskit');
const gpuiDir = path.join(repoRoot, 'tmp', 'scenes_gpui');

const defaultDiff = {
  max_mean_abs: 2.5,
  max_rmse: 12,
  max_delta: 96,
  changed_ratio_max: 0.08,
  per_channel_tol: 2,
  ignore_alpha: false,
};

function loadCatalog() {
  if (!fs.existsSync(catalogPath)) {
    return {
      name: 'standard',
      oracle: 'mixed',
      case_count: 0,
      default_diff: defaultDiff,
      images_dir: 'standard',
      cases: {},
    };
  }
  return JSON.parse(fs.readFileSync(catalogPath, 'utf8'));
}

function pngSize(file) {
  const b = fs.readFileSync(file);
  // IHDR width/height at bytes 16-23 big-endian
  return { w: b.readUInt32BE(16), h: b.readUInt32BE(20) };
}

// crude mean abs RGB (slow but fine for a few images)
function meanAbs(aPath, bPath) {
  // use sharp-less pure approach via pngjs if available; else skip and return 999
  try {
    const PNG = require('pngjs').PNG;
    const a = PNG.sync.read(fs.readFileSync(aPath));
    const b = PNG.sync.read(fs.readFileSync(bPath));
    if (a.width !== b.width || a.height !== b.height) return { mean: 999, rmse: 999, ratio: 1, maxd: 255, w: a.width, h: a.height, bw: b.width, bh: b.height };
    let sum = 0, sumSq = 0, changed = 0, maxd = 0, n = a.width * a.height;
    for (let i = 0; i < n; i++) {
      const o = i * 4;
      for (let c = 0; c < 3; c++) {
        const d = Math.abs(a.data[o + c] - b.data[o + c]);
        sum += d; sumSq += d * d;
        if (d > maxd) maxd = d;
        if (d > 2) changed = changed; // per pixel below
      }
      if (Math.abs(a.data[o]-b.data[o])>2 || Math.abs(a.data[o+1]-b.data[o+1])>2 || Math.abs(a.data[o+2]-b.data[o+2])>2) changed++;
    }
    return { mean: sum / (n * 3), rmse: Math.sqrt(sumSq / (n * 3)), ratio: changed / n, maxd, w: a.width, h: a.height };
  } catch (e) {
    return null;
  }
}

async function fromScenes() {
  const files = fs.readdirSync(scenesDir).filter((f) => f.endsWith('.json'));
  const cat = loadCatalog();
  if (!cat.cases) cat.cases = {};
  cat.images_dir = 'standard';
  cat.generated_at = new Date().toISOString();
  fs.mkdirSync(imagesDir, { recursive: true });
  fs.mkdirSync(ckDir, { recursive: true });

  for (const f of files) {
    const sp = path.join(scenesDir, f);
    const scene = JSON.parse(fs.readFileSync(sp, 'utf8'));
    const ckOut = path.join(ckDir, scene.id + '.png');
    await renderSceneToPNG(sp, ckOut, { fontRoot: repoRoot });
    const gpuiOut = path.join(gpuiDir, scene.id + '.png');
    const hasText = (scene.ops || []).some((o) => o.op === 'fill_text');
    const tol = hasText
      ? { ...defaultDiff, max_mean_abs: 6, max_rmse: 20, changed_ratio_max: 0.15, max_delta: 128 }
      : { ...defaultDiff, max_mean_abs: 4, max_rmse: 16, changed_ratio_max: 0.12, max_delta: 96 };

    let oracle = 'canvaskit';
    let chosen = ckOut;
    let note = 'scene: testdata/scenes/' + f + ' (CanvasKit)';
    if (fs.existsSync(gpuiOut)) {
      const stats = meanAbs(ckOut, gpuiOut);
      if (!stats) {
        // no pngjs — prefer gpui-scene for reliability when gpui present
        oracle = 'gpui-scene';
        chosen = gpuiOut;
        note = 'scene: testdata/scenes/' + f + ' (gpui-scene; install pngjs for CK auto-pick)';
      } else if (stats.mean <= tol.max_mean_abs && stats.rmse <= tol.max_rmse && stats.ratio <= tol.changed_ratio_max) {
        oracle = 'canvaskit';
        chosen = ckOut;
        note = `scene + CanvasKit (vs gpui mean=${stats.mean.toFixed(3)})`;
      } else {
        oracle = 'gpui-scene';
        chosen = gpuiOut;
        note = `scene + gpui-scene (CK drift mean=${stats.mean.toFixed(3)} rmse=${stats.rmse.toFixed(3)})`;
        console.log('  CK drift', scene.id, stats);
      }
    }

    const dest = path.join(imagesDir, scene.id + '.png');
    fs.copyFileSync(chosen, dest);
    const sz = pngSize(dest);
    cat.cases[scene.id] = {
      id: scene.id,
      oracle,
      width: sz.w,
      height: sz.h,
      diff: tol,
      note,
      file: scene.id + '.png',
    };
    console.log(oracle, dest);
  }
  cat.case_count = Object.keys(cat.cases).length;
  // overall oracle label
  const oracles = new Set(Object.values(cat.cases).map((c) => c.oracle));
  cat.oracle = oracles.size === 1 ? [...oracles][0] : 'mixed';
  cat.oracle_note = 'Scene-driven standard data. Prefer CanvasKit when within tolerance; else gpui-scene from same script.';
  fs.writeFileSync(catalogPath, JSON.stringify(cat, null, 2) + '\n');
  console.log('updated', catalogPath, 'cases', cat.case_count, 'oracle', cat.oracle);
}

async function main() {
  const args = process.argv.slice(2);
  if (args[0] === '--check') {
    const j = JSON.parse(fs.readFileSync(catalogPath, 'utf8'));
    const pngs = fs.readdirSync(imagesDir).filter((f) => f.endsWith('.png'));
    console.log(JSON.stringify({ case_count: j.case_count, png_count: pngs.length, oracle: j.oracle, ok: pngs.length > 0 }, null, 2));
    return;
  }
  if (args[0] === '--from-scenes') {
    await fromScenes();
    return;
  }
  console.log('Usage: node gen.mjs --from-scenes | --check');
  process.exit(1);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
