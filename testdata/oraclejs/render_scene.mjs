/**
 * Render one scene JSON with CanvasKit and write PNG.
 * Shared op set with stdgate/scene (Go).
 */
import fs from 'fs';
import path from 'path';
import { createRequire } from 'module';
import { fileURLToPath } from 'url';

const require = createRequire(import.meta.url);
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, '../..');

function rgbaBytes(rgba, fallback = [0, 0, 0, 1]) {
  const v = rgba && rgba.length ? rgba : fallback;
  const r = Math.round(clamp01(v[0] ?? 0) * 255);
  const g = Math.round(clamp01(v[1] ?? 0) * 255);
  const b = Math.round(clamp01(v[2] ?? 0) * 255);
  const a = clamp01(v[3] ?? 1);
  return [r, g, b, a];
}

function clamp01(x) {
  return Math.max(0, Math.min(1, Number(x)));
}

function color4f(rgba, fallback) {
  const [r, g, b, a] = (() => {
    const v = rgba && rgba.length ? rgba : fallback;
    return [clamp01(v[0] ?? 0), clamp01(v[1] ?? 0), clamp01(v[2] ?? 0), clamp01(v[3] ?? 1)];
  })();
  return [r, g, b, a];
}

function blendMode(CK, name) {
  switch ((name || 'normal').toLowerCase()) {
    case 'plus':
    case 'add':
      return CK.BlendMode.Plus;
    case 'multiply':
      return CK.BlendMode.Multiply;
    case 'screen':
      return CK.BlendMode.Screen;
    default:
      return CK.BlendMode.SrcOver;
  }
}

export async function loadCanvasKit() {
  const init = (await import('canvaskit-wasm')).default;
  // Resolve wasm next to the installed package without using package.json exports.
  const wasmPath = path.join(__dirname, 'node_modules', 'canvaskit-wasm', 'bin', 'canvaskit.wasm');
  return init({
    locateFile: (file) => {
      if (file.endsWith('.wasm')) return wasmPath;
      return path.join(path.dirname(wasmPath), file);
    },
  });
}

export async function renderSceneToPNG(scenePath, outPath, { fontRoot = repoRoot } = {}) {
  const CK = await loadCanvasKit();
  const scene = JSON.parse(fs.readFileSync(scenePath, 'utf8'));
  // size = logical pixels (same as gpui NewContext width/height).
  // physical PNG = size * scale (same as gpui PixelWidth/Height).
  const lw = scene.size[0];
  const lh = scene.size[1];
  const scale = scene.scale && scene.scale > 0 ? scene.scale : 1;
  const w = Math.round(lw * scale);
  const h = Math.round(lh * scale);

  const surface = CK.MakeSurface(w, h);
  if (!surface) throw new Error('MakeSurface failed');
  const canvas = surface.getCanvas();
  canvas.clear(CK.Color(255, 255, 255, 1));
  if (scale !== 1) {
    canvas.scale(scale, scale);
  }

  let font = null;
  let typeface = null;
  if (scene.font && scene.font.path) {
    let fp = scene.font.path;
    if (!path.isAbsolute(fp)) fp = path.join(fontRoot, fp);
    const bytes = fs.readFileSync(fp);
    typeface = CK.Typeface.MakeFreeTypeFaceFromData(bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength));
    const sz = scene.font.size || 16;
    font = new CK.Font(typeface, sz);
  }

  const paint = new CK.Paint();
  paint.setAntiAlias(true);

  for (const op of scene.ops || []) {
    switch (op.op) {
      case 'clear': {
        const c = color4f(op.rgba, [1, 1, 1, 1]);
        canvas.clear(CK.Color(c[0] * 255, c[1] * 255, c[2] * 255, c[3]));
        break;
      }
      case 'fill_rect': {
        const [x, y, rw, rh] = op.rect;
        const c = color4f(op.rgba, [0, 0, 0, 1]);
        paint.setStyle(CK.PaintStyle.Fill);
        paint.setColor(CK.Color(c[0] * 255, c[1] * 255, c[2] * 255, c[3]));
        paint.setBlendMode(CK.BlendMode.SrcOver);
        canvas.drawRect([x, y, x + rw, y + rh], paint);
        break;
      }
      case 'fill_rrect': {
        const [x, y, rw, rh] = op.rect;
        const c = color4f(op.rgba, [0, 0, 0, 1]);
        paint.setStyle(CK.PaintStyle.Fill);
        paint.setColor(CK.Color(c[0] * 255, c[1] * 255, c[2] * 255, c[3]));
        paint.setBlendMode(CK.BlendMode.SrcOver);
        const rr = CK.RRectXY([x, y, x + rw, y + rh], op.radius || 0, op.radius || 0);
        canvas.drawRRect(rr, paint);
        break;
      }
      case 'fill_circle': {
        const c = color4f(op.rgba, [0, 0, 0, 1]);
        paint.setStyle(CK.PaintStyle.Fill);
        paint.setColor(CK.Color(c[0] * 255, c[1] * 255, c[2] * 255, c[3]));
        canvas.drawCircle(op.cx, op.cy, op.radius, paint);
        break;
      }
      case 'stroke_line': {
        const c = color4f(op.rgba, [0, 0, 0, 1]);
        paint.setStyle(CK.PaintStyle.Stroke);
        paint.setColor(CK.Color(c[0] * 255, c[1] * 255, c[2] * 255, c[3]));
        // hairline: width 0 → 1 device px
        paint.setStrokeWidth(op.width > 0 ? op.width : 1 / scale);
        canvas.drawLine(op.x, op.y, op.x2, op.y2, paint);
        paint.setStyle(CK.PaintStyle.Fill);
        break;
      }
      case 'clip_rect': {
        const [x, y, rw, rh] = op.rect;
        canvas.clipRect([x, y, x + rw, y + rh], CK.ClipOp.Intersect, true);
        break;
      }
      case 'clip_rrect': {
        const [x, y, rw, rh] = op.rect;
        const rr = CK.RRectXY([x, y, x + rw, y + rh], op.radius || 0, op.radius || 0);
        canvas.clipRRect(rr, CK.ClipOp.Intersect, true);
        break;
      }
      case 'clip_path': {
        const p = new CK.Path();
        const pts = op.points || [];
        if (pts.length) {
          p.moveTo(pts[0][0], pts[0][1]);
          for (let i = 1; i < pts.length; i++) p.lineTo(pts[i][0], pts[i][1]);
          if (op.close) p.close();
        }
        canvas.clipPath(p, CK.ClipOp.Intersect, true);
        p.delete();
        break;
      }
      case 'reset_clip': {
        // CanvasKit has no full reset; restore to identity clip via save/restore not matching gpui.
        // Approximate: clip to full logical canvas.
        const logicalW = scene.size[0];
        const logicalH = scene.size[1];
        canvas.clipRect([0, 0, logicalW, logicalH], CK.ClipOp.Intersect, false);
        break;
      }
      case 'layer_begin': {
        const opac = op.opacity > 0 ? op.opacity : 1;
        const paintLayer = new CK.Paint();
        paintLayer.setAlphaf(opac);
        paintLayer.setBlendMode(blendMode(CK, op.blend));
        paintLayer.setAntiAlias(true);
        canvas.saveLayer(paintLayer);
        paintLayer.delete();
        break;
      }
      case 'layer_end': {
        canvas.restore();
        break;
      }
      case 'set_blend': {
        paint.setBlendMode(blendMode(CK, op.blend));
        break;
      }
      case 'fill_text': {
        if (!font) break;
        const c = color4f(op.rgba, [0, 0, 0, 1]);
        paint.setStyle(CK.PaintStyle.Fill);
        paint.setColor(CK.Color(c[0] * 255, c[1] * 255, c[2] * 255, c[3]));
        // CanvasKit drawText uses baseline similar to fillText
        canvas.drawText(op.text || '', op.x, op.y, paint, font);
        break;
      }
      case 'push':
        canvas.save();
        break;
      case 'pop':
        canvas.restore();
        break;
      case 'translate':
        canvas.translate(op.x || 0, op.y || 0);
        break;
      case 'scale': {
        const sx = op.x ?? 1;
        const sy = op.y ?? sx;
        canvas.scale(sx, sy);
        break;
      }
      case 'rotate':
        canvas.rotate((op.angle || 0) * 180 / Math.PI, 0, 0);
        break;
      case 'draw_image_solid': {
        const iw = op.img_w || op.w || 1;
        const ih = op.img_h || op.h || 1;
        const [R, G, B, A] = rgbaBytes(op.rgba, [0, 0, 0, 1]);
        const pixels = new Uint8Array(iw * ih * 4);
        for (let i = 0; i < iw * ih; i++) {
          pixels[i * 4] = R;
          pixels[i * 4 + 1] = G;
          pixels[i * 4 + 2] = B;
          pixels[i * 4 + 3] = Math.round(A * 255);
        }
        const img = CK.MakeImage({
          width: iw,
          height: ih,
          alphaType: CK.AlphaType.Unpremul,
          colorType: CK.ColorType.RGBA_8888,
          colorSpace: CK.ColorSpace.SRGB,
        }, pixels, iw * 4);
        if (img) {
          canvas.drawImage(img, op.x || 0, op.y || 0);
          img.delete();
        }
        break;
      }
      default:
        throw new Error(`unsupported op ${op.op}`);
    }
  }

  // flush
  surface.flush();
  const img = surface.makeImageSnapshot();
  const png = img.encodeToBytes();
  if (!png) throw new Error('encodeToBytes failed');
  fs.mkdirSync(path.dirname(outPath), { recursive: true });
  fs.writeFileSync(outPath, Buffer.from(png));
  img.delete();
  surface.delete();
  paint.delete();
  if (font) font.delete();
  // typeface may be owned by font
  return outPath;
}

// CLI
if (import.meta.url === `file://${process.argv[1]}` || process.argv[1] && process.argv[1].endsWith('render_scene.mjs')) {
  const args = process.argv.slice(2);
  const sceneIdx = args.indexOf('--scene');
  const dirIdx = args.indexOf('--dir');
  const outIdx = args.indexOf('--out');
  const outDir = outIdx >= 0 ? args[outIdx + 1] : path.join(repoRoot, 'tmp', 'scenes_canvaskit');
  const jobs = [];
  if (sceneIdx >= 0) jobs.push(args[sceneIdx + 1]);
  if (dirIdx >= 0) {
    const d = args[dirIdx + 1];
    for (const f of fs.readdirSync(d)) {
      if (f.endsWith('.json')) jobs.push(path.join(d, f));
    }
  }
  if (!jobs.length) {
    console.error('Usage: node render_scene.mjs --scene file.json|--dir dir --out outDir');
    process.exit(1);
  }
  for (const j of jobs) {
    const scene = JSON.parse(fs.readFileSync(j, 'utf8'));
    const out = path.join(outDir, scene.id + '.png');
    await renderSceneToPNG(j, out);
    console.log('wrote', out);
  }
}
