/**
 * Render one scene JSON with CanvasKit and write PNG.
 * Shared op set with stdgate/scene (Go).
 * Pixel standards for D cases are produced only by this oracle.
 */
import fs from 'fs';
import path from 'path';
import { createRequire } from 'module';
import { fileURLToPath } from 'url';

const require = createRequire(import.meta.url);
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, '../..');

function color4f(rgba, def) {
  if (!rgba || rgba.length < 3) return def;
  return [rgba[0], rgba[1], rgba[2], rgba.length >= 4 ? rgba[3] : 1];
}

function rgbaBytes(rgba, def) {
  const c = color4f(rgba, def);
  return [
    Math.round(Math.min(1, Math.max(0, c[0])) * 255),
    Math.round(Math.min(1, Math.max(0, c[1])) * 255),
    Math.round(Math.min(1, Math.max(0, c[2])) * 255),
    Math.min(1, Math.max(0, c[3])),
  ];
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

function makePath(CK, op) {
  const p = new CK.Path();
  const pts = op.points || [];
  if (pts.length) {
    p.moveTo(pts[0][0], pts[0][1]);
    for (let i = 1; i < pts.length; i++) {
      if (pts[i] && pts[i].length >= 2) p.lineTo(pts[i][0], pts[i][1]);
    }
    if (op.close) p.close();
  }
  return p;
}

function applyDash(CK, paint, dash) {
  if (dash && dash.length) {
    const pe = CK.PathEffect.MakeDash(dash, 0);
    if (pe) paint.setPathEffect(pe);
  } else {
    paint.setPathEffect(null);
  }
}

export async function loadCanvasKit() {
  const init = (await import('canvaskit-wasm')).default;
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
  const lw = scene.size[0];
  const lh = scene.size[1];
  const scale = scene.scale && scene.scale > 0 ? scene.scale : 1;
  const w = Math.round(lw * scale);
  const h = Math.round(lh * scale);

  const surface = CK.MakeSurface(w, h);
  if (!surface) throw new Error('MakeSurface failed');
  const canvas = surface.getCanvas();
  canvas.clear(CK.Color(255, 255, 255, 1));
  if (scale !== 1) canvas.scale(scale, scale);

  let font = null;
  let typeface = null;
  // default font for scenes that use text without font block
  const defaultFontPath = path.join(fontRoot, 'standardtest/fonts/DejaVuSans.ttf');
  const loadFont = (fp, sz) => {
    const bytes = fs.readFileSync(fp);
    typeface = CK.Typeface.MakeFreeTypeFaceFromData(
      bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength),
    );
    return new CK.Font(typeface, sz || 16);
  };
  if (scene.font && scene.font.path) {
    let fp = scene.font.path;
    if (!path.isAbsolute(fp)) fp = path.join(fontRoot, fp);
    font = loadFont(fp, scene.font.size || 16);
  } else if (fs.existsSync(defaultFontPath)) {
    font = loadFont(defaultFontPath, 16);
  }

  const paint = new CK.Paint();
  paint.setAntiAlias(true);
  paint.setBlendMode(CK.BlendMode.SrcOver);

  // mask state: between set_mask_alpha and clear_mask, capture into a layer then DstIn
  let maskImage = null;
  let maskLayerOpen = false;

  const openMaskLayer = () => {
    if (maskLayerOpen) return;
    canvas.saveLayer();
    maskLayerOpen = true;
  };
  const closeMaskLayer = () => {
    if (!maskLayerOpen) return;
    if (maskImage) {
      const mp = new CK.Paint();
      mp.setBlendMode(CK.BlendMode.DstIn);
      mp.setAntiAlias(true);
      canvas.drawImage(maskImage, 0, 0, mp);
      mp.delete();
    }
    canvas.restore();
    maskLayerOpen = false;
  };

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
        paint.setPathEffect(null);
        paint.setColor(CK.Color(c[0] * 255, c[1] * 255, c[2] * 255, c[3]));
        canvas.drawRect([x, y, x + rw, y + rh], paint);
        break;
      }
      case 'fill_rrect': {
        const [x, y, rw, rh] = op.rect;
        const c = color4f(op.rgba, [0, 0, 0, 1]);
        paint.setStyle(CK.PaintStyle.Fill);
        paint.setPathEffect(null);
        paint.setColor(CK.Color(c[0] * 255, c[1] * 255, c[2] * 255, c[3]));
        const rr = CK.RRectXY([x, y, x + rw, y + rh], op.radius || 0, op.radius || 0);
        canvas.drawRRect(rr, paint);
        break;
      }
      case 'fill_circle': {
        const c = color4f(op.rgba, [0, 0, 0, 1]);
        paint.setStyle(CK.PaintStyle.Fill);
        paint.setPathEffect(null);
        paint.setColor(CK.Color(c[0] * 255, c[1] * 255, c[2] * 255, c[3]));
        canvas.drawCircle(op.cx, op.cy, op.radius, paint);
        break;
      }
      case 'fill_path': {
        const c = color4f(op.rgba, [0, 0, 0, 1]);
        paint.setStyle(CK.PaintStyle.Fill);
        paint.setPathEffect(null);
        paint.setColor(CK.Color(c[0] * 255, c[1] * 255, c[2] * 255, c[3]));
        const p = makePath(CK, op);
        canvas.drawPath(p, paint);
        p.delete();
        break;
      }
      case 'stroke_line': {
        const c = color4f(op.rgba, [0, 0, 0, 1]);
        paint.setStyle(CK.PaintStyle.Stroke);
        paint.setColor(CK.Color(c[0] * 255, c[1] * 255, c[2] * 255, c[3]));
        paint.setStrokeWidth(op.width > 0 ? op.width : 1 / scale);
        applyDash(CK, paint, op.dash);
        canvas.drawLine(op.x, op.y, op.x2, op.y2, paint);
        paint.setPathEffect(null);
        paint.setStyle(CK.PaintStyle.Fill);
        break;
      }
      case 'stroke_rect': {
        const [x, y, rw, rh] = op.rect;
        const c = color4f(op.rgba, [0, 0, 0, 1]);
        paint.setStyle(CK.PaintStyle.Stroke);
        paint.setColor(CK.Color(c[0] * 255, c[1] * 255, c[2] * 255, c[3]));
        paint.setStrokeWidth(op.width > 0 ? op.width : 1);
        applyDash(CK, paint, op.dash);
        canvas.drawRect([x, y, x + rw, y + rh], paint);
        paint.setPathEffect(null);
        paint.setStyle(CK.PaintStyle.Fill);
        break;
      }
      case 'stroke_rrect': {
        const [x, y, rw, rh] = op.rect;
        const c = color4f(op.rgba, [0, 0, 0, 1]);
        paint.setStyle(CK.PaintStyle.Stroke);
        paint.setColor(CK.Color(c[0] * 255, c[1] * 255, c[2] * 255, c[3]));
        paint.setStrokeWidth(op.width > 0 ? op.width : 1);
        applyDash(CK, paint, op.dash);
        const rr = CK.RRectXY([x, y, x + rw, y + rh], op.radius || 0, op.radius || 0);
        canvas.drawRRect(rr, paint);
        paint.setPathEffect(null);
        paint.setStyle(CK.PaintStyle.Fill);
        break;
      }
      case 'stroke_circle': {
        const c = color4f(op.rgba, [0, 0, 0, 1]);
        paint.setStyle(CK.PaintStyle.Stroke);
        paint.setColor(CK.Color(c[0] * 255, c[1] * 255, c[2] * 255, c[3]));
        paint.setStrokeWidth(op.width > 0 ? op.width : 1);
        applyDash(CK, paint, op.dash);
        canvas.drawCircle(op.cx, op.cy, op.radius, paint);
        paint.setPathEffect(null);
        paint.setStyle(CK.PaintStyle.Fill);
        break;
      }
      case 'stroke_path': {
        const c = color4f(op.rgba, [0, 0, 0, 1]);
        paint.setStyle(CK.PaintStyle.Stroke);
        paint.setColor(CK.Color(c[0] * 255, c[1] * 255, c[2] * 255, c[3]));
        paint.setStrokeWidth(op.width > 0 ? op.width : 1);
        applyDash(CK, paint, op.dash);
        const p = makePath(CK, op);
        canvas.drawPath(p, paint);
        p.delete();
        paint.setPathEffect(null);
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
        const p = makePath(CK, op);
        canvas.clipPath(p, CK.ClipOp.Intersect, true);
        p.delete();
        break;
      }
      case 'reset_clip': {
        // Approximate full reset: intersect with full logical canvas (CanvasKit has no clip reset).
        canvas.clipRect([0, 0, lw, lh], CK.ClipOp.Intersect, false);
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
        paint.setPathEffect(null);
        paint.setColor(CK.Color(c[0] * 255, c[1] * 255, c[2] * 255, c[3]));
        if (op.font_size && typeface) {
          const f2 = new CK.Font(typeface, op.font_size);
          canvas.drawText(op.text || '', op.x, op.y, paint, f2);
          f2.delete();
        } else {
          canvas.drawText(op.text || '', op.x, op.y, paint, font);
        }
        break;
      }
      case 'stroke_text': {
        if (!font) break;
        const c = color4f(op.rgba, [0, 0, 0, 1]);
        paint.setStyle(CK.PaintStyle.Stroke);
        paint.setPathEffect(null);
        paint.setStrokeWidth(op.width > 0 ? op.width : 1);
        paint.setColor(CK.Color(c[0] * 255, c[1] * 255, c[2] * 255, c[3]));
        canvas.drawText(op.text || '', op.x, op.y, paint, font);
        paint.setStyle(CK.PaintStyle.Fill);
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
        canvas.rotate(((op.angle || 0) * 180) / Math.PI, 0, 0);
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
        const img = CK.MakeImage(
          {
            width: iw,
            height: ih,
            alphaType: CK.AlphaType.Unpremul,
            colorType: CK.ColorType.RGBA_8888,
            colorSpace: CK.ColorSpace.SRGB,
          },
          pixels,
          iw * 4,
        );
        if (img) {
          canvas.drawImage(img, op.x || 0, op.y || 0);
          img.delete();
        }
        break;
      }
      case 'set_mask_alpha': {
        // Close previous mask layer if any
        closeMaskLayer();
        if (maskImage) {
          maskImage.delete();
          maskImage = null;
        }
        const mw = op.w || op.img_w;
        const mh = op.h || op.img_h;
        if (!op.b64 || !mw || !mh) break;
        const raw = Buffer.from(op.b64, 'base64');
        // Expand A plane to RGBA (white * alpha) for DstIn
        const pixels = new Uint8Array(mw * mh * 4);
        for (let i = 0; i < mw * mh; i++) {
          const a = raw[i] || 0;
          pixels[i * 4] = 255;
          pixels[i * 4 + 1] = 255;
          pixels[i * 4 + 2] = 255;
          pixels[i * 4 + 3] = a;
        }
        maskImage = CK.MakeImage(
          {
            width: mw,
            height: mh,
            alphaType: CK.AlphaType.Unpremul,
            colorType: CK.ColorType.RGBA_8888,
            colorSpace: CK.ColorSpace.SRGB,
          },
          pixels,
          mw * 4,
        );
        openMaskLayer();
        break;
      }
      case 'clear_mask': {
        closeMaskLayer();
        if (maskImage) {
          maskImage.delete();
          maskImage = null;
        }
        break;
      }
      default:
        throw new Error(`unsupported op ${op.op}`);
    }
  }

  // flush pending mask
  closeMaskLayer();
  if (maskImage) maskImage.delete();

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
  return outPath;
}

// CLI
const isMain =
  import.meta.url === `file://${process.argv[1]}` ||
  (process.argv[1] && process.argv[1].endsWith('render_scene.mjs'));
if (isMain) {
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
