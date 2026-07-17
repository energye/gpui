#!/usr/bin/env node
/**
 * Generate CanvasKit standard PNGs into standardtest/diff/standard.
 *
 *   node gen.mjs --from-scenes --scenes <dir> --out <dir> --font-root <root>
 *   node gen.mjs --from-scenes --id D01_ClipLayerText ...
 */
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import { renderSceneToPNG } from './render_scene.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

function arg(name, def) {
  const i = process.argv.indexOf(name);
  if (i >= 0 && process.argv[i + 1]) return process.argv[i + 1];
  return def;
}
function has(name) {
  return process.argv.includes(name);
}

async function main() {
  if (!has('--from-scenes')) {
    console.log('Usage: node gen.mjs --from-scenes --scenes DIR --out DIR --font-root DIR [--id ID]');
    process.exit(2);
  }
  const scenesDir = path.resolve(arg('--scenes', path.join(__dirname, '../scenes')));
  const outDir = path.resolve(arg('--out', path.join(__dirname, '../diff/standard')));
  const fontRoot = path.resolve(arg('--font-root', path.join(__dirname, '../..')));
  const onlyID = arg('--id', '');

  fs.mkdirSync(outDir, { recursive: true });
  let files = fs.readdirSync(scenesDir).filter((f) => f.endsWith('.json')).sort();
  if (onlyID) {
    files = files.filter((f) => f === onlyID + '.json' || f.startsWith(onlyID));
  }

  let ok = 0;
  let fail = 0;
  for (const f of files) {
    const sp = path.join(scenesDir, f);
    let scene;
    try {
      scene = JSON.parse(fs.readFileSync(sp, 'utf8'));
    } catch (e) {
      fail++;
      console.error('FAIL load', f, e.message);
      continue;
    }
    const id = scene.id || path.basename(f, '.json');
    if (onlyID && id !== onlyID) continue;
    const out = path.join(outDir, id + '.png');
    try {
      await renderSceneToPNG(sp, out, { fontRoot });
      ok++;
      if (ok % 25 === 0 || ok === 1) console.log(`  standard ${ok}/${files.length} ${id}`);
    } catch (e) {
      fail++;
      console.error('FAIL', id, e.message || e);
    }
  }
  console.log(`canvaskit standards ok=${ok} fail=${fail} → ${outDir}`);
  if (fail) process.exit(1);
}

await main();
