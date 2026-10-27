// P3/P4 Flutter comparison ruler.
//
// Scenes, selected with RULER_SCENE:
//   spin     — 480x320, one 64px rotating icon (matrix row 2 / spin_min)
//   list     — 1200x800, 1200 variable-height rows + static panels (matrix row 3)
//   text     — 1200x800, multiscript/emoji/rich/multi-size dense page (row 12)
//   full     — 1200x800, full-window repaint every frame (row 4 / R0)
//   grid     — 1200x800, 4x5 async-decoded tiles light up (row 5 / R10)
//   boundary — 1200x800, 4x4 RepaintBoundary grid, rotating hot cells (row 6 / R3b)
//   overlay  — 1200x800, dimming panel opens at 1.2s and holds (row 8 / R8)
//   resize   — 1200x800, responsive layout; runner resize waves via RULER_ACTIONS (row 7 / R11)
//   min      — 1200x800, FG panel + static grid; runner self-minimizes @22 (row 9 / R17)
//   cp       — 1200x800, 100 animated bezier strokes (row 13 vector arm)
//
// The process prints machine-readable RULER_METRIC lines. CPU/RSS are sampled
// externally from /proc (see scripts/sample.sh), matching the gpui ruler.

import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:math' as math;
import 'dart:ui' show FramePhase;
import 'dart:ui' as ruler_ui show Image, instantiateImageCodec;

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart' show ScrollCacheExtent;
import 'package:flutter/scheduler.dart';
import 'package:flutter/services.dart';

const Color kBg = Color(0xFF1A1E29);
const Color kPanel = Color(0xFF202634);
const Color kPanel2 = Color(0xFF2A3142);
const Color kInk = Color(0xFFE2E7F2);
const Color kMuted = Color(0xFF9AA6BD);

late final String kScene;
late final int kRunSeconds;

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  kScene = Platform.environment['RULER_SCENE'] ?? 'spin';
  kRunSeconds = int.tryParse(Platform.environment['RULER_SECONDS'] ?? '0') ?? 0;

  final corpus = await TextCorpus.load();
  final meter = FrameMeter(kScene);
  SchedulerBinding.instance.addTimingsCallback(meter.onTimings);
  meter.start();

  runApp(RulerApp(scene: kScene, corpus: corpus));

  SchedulerBinding.instance.addPostFrameCallback((_) {
    final view = SchedulerBinding.instance.platformDispatcher.views.first;
    final s = view.physicalSize;
    stdout.writeln(
      'RULER_WINDOW scene=$kScene physical=${s.width.toStringAsFixed(1)}x'
      '${s.height.toStringAsFixed(1)} dpr=${view.devicePixelRatio}',
    );
  });

  if (kRunSeconds > 0) {
    Timer(Duration(seconds: kRunSeconds), () async {
      meter.reportFinal();
      await stdout.flush();
      exit(0);
    });
  }
}

class RulerApp extends StatelessWidget {
  const RulerApp({super.key, required this.scene, required this.corpus});

  final String scene;
  final TextCorpus corpus;

  @override
  Widget build(BuildContext context) {
    Widget home;
    switch (scene) {
      case 'list':
        home = const ListScene();
        break;
      case 'text':
        home = TextScene(corpus: corpus);
        break;
      case 'full':
        home = const FullScene();
        break;
      case 'grid':
        home = const GridScene();
        break;
      case 'boundary':
        home = const BoundaryScene();
        break;
      case 'overlay':
        home = const OverlayScene();
        break;
      case 'resize':
        home = const ResizeScene();
        break;
      case 'min':
        home = const MinScene();
        break;
      case 'cp':
        home = const CPScene();
        break;
      default:
        home = const SpinScene();
        break;
    }
    return MaterialApp(
      debugShowCheckedModeBanner: false,
      title: 'Flutter ruler — $scene',
      theme: ThemeData(
        brightness: Brightness.dark,
        scaffoldBackgroundColor: kBg,
        fontFamily: 'Roboto',
        useMaterial3: true,
      ),
      home: Scaffold(body: home),
    );
  }
}

// ---------------------------------------------------------------------------
// Frame timing / fps evidence
// ---------------------------------------------------------------------------

class FrameMeter {
  FrameMeter(this.scene);

  final String scene;
  final Stopwatch _wall = Stopwatch()..start();
  final List<double> _intervals = <double>[];
  final List<double> _raster = <double>[];
  Timer? _timer;
  int _frames = 0;
  int _lastVsyncUs = 0;
  int _reportFrames = 0;
  int _reportUs = 0;
  bool _final = false;

  void start() {
    _reportUs = _wall.elapsedMicroseconds;
    stdout.writeln('RULER_READY scene=$scene pid=$pid');
    _timer = Timer.periodic(const Duration(seconds: 2), (_) => _emit());
  }

  void onTimings(List<FrameTiming> timings) {
    for (final t in timings) {
      final us = t.timestampInMicroseconds(FramePhase.vsyncStart);
      if (_lastVsyncUs > 0 && us > _lastVsyncUs) {
        final ms = (us - _lastVsyncUs) / 1000.0;
        if (ms > 0 && ms < 2000) _intervals.add(ms);
      }
      _lastVsyncUs = us;
      _frames++;
      _raster.add(t.rasterDuration.inMicroseconds / 1000.0);
    }
  }

  void _emit() {
    if (_final) return;
    final nowUs = _wall.elapsedMicroseconds;
    final dt = (nowUs - _reportUs) / 1e6;
    final n = _frames - _reportFrames;
    final fps = dt > 0 ? n / dt : 0.0;
    final p95 = _p95(_intervals);
    final hitch = _intervals.where((x) => x > 33.0).length;
    final rasterP95 = _p95(_raster);
    stdout.writeln(jsonEncode({
      'event': 'RULER_METRIC',
      'scene': scene,
      't_s': (nowUs / 1e6).toStringAsFixed(1),
      'frames': n,
      'fps': double.parse(fps.toStringAsFixed(1)),
      'p95_ms': double.parse(p95.toStringAsFixed(2)),
      'raster_p95_ms': double.parse(rasterP95.toStringAsFixed(2)),
      'hitch': hitch,
    }));
    _intervals.clear();
    _raster.clear();
    _reportFrames = _frames;
    _reportUs = nowUs;
  }

  void reportFinal() {
    _timer?.cancel();
    _final = true;
    final n = _frames;
    final dt = _wall.elapsedMicroseconds / 1e6;
    final fps = dt > 0 ? n / dt : 0.0;
    final p95 = _p95(_intervals);
    final rasterP95 = _p95(_raster);
    stdout.writeln(jsonEncode({
      'event': 'RULER_FINAL',
      'scene': scene,
      't_s': dt.toStringAsFixed(1),
      'frames': n,
      'fps_avg': double.parse(fps.toStringAsFixed(2)),
      'p95_ms': double.parse(p95.toStringAsFixed(2)),
      'raster_p95_ms': double.parse(rasterP95.toStringAsFixed(2)),
    }));
  }

  static double _p95(List<double> xs) {
    if (xs.isEmpty) return 0;
    final ys = List<double>.from(xs)..sort();
    final i = ((ys.length - 1) * 0.95).round();
    return ys[i];
  }
}

// ---------------------------------------------------------------------------
// Scene 1: spin (480x320)
// ---------------------------------------------------------------------------

class SpinScene extends StatefulWidget {
  const SpinScene({super.key});

  @override
  State<SpinScene> createState() => _SpinSceneState();
}

class _SpinSceneState extends State<SpinScene>
    with SingleTickerProviderStateMixin {
  late final AnimationController _spin = AnimationController(
    vsync: this,
    duration: const Duration(seconds: 2),
  )..repeat();

  @override
  void dispose() {
    _spin.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return ColoredBox(
      color: const Color(0xFF1A1E29),
      child: Center(
        child: RotationTransition(
          turns: _spin,
          child: const Icon(Icons.sync, size: 64, color: Colors.white),
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Text corpus and scene 3: dense text (1200x800)
// ---------------------------------------------------------------------------

class TextCorpus {
  const TextCorpus(this.multi, this.emoji, this.rich);

  final String multi;
  final String emoji;
  final String rich;

  static Future<TextCorpus> load() async {
    final values = await Future.wait([
      rootBundle.loadString('assets/multiscript.txt'),
      rootBundle.loadString('assets/emoji.txt'),
      rootBundle.loadString('assets/richtext.txt'),
    ]);
    return TextCorpus(values[0], values[1], values[2]);
  }
}

class TextScene extends StatelessWidget {
  const TextScene({super.key, required this.corpus});

  final TextCorpus corpus;

  @override
  Widget build(BuildContext context) {
    final richLines = corpus.rich.trim().split('\n');
    const sizes = [20.0, 14.0, 8.0, 12.0];
    const colors = [
      Color(0xFFF2E680),
      Color(0xFFD9E0EB),
      Color(0xFF99CCF2),
      Color(0xFFBFF2BF),
    ];

    Widget label(String s) => Padding(
          padding: const EdgeInsets.only(bottom: 4),
          child: Text(
            s,
            style: const TextStyle(fontSize: 11, color: Color(0xFF8CBFF2)),
          ),
        );

    Widget body(String s) => SizedBox(
          width: 560,
          child: Text(
            s,
            style: const TextStyle(fontSize: 14, color: kInk, height: 1.2),
          ),
        );

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _Header('Flutter ruler — dense text (M5 semantic)'),
        Expanded(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(12, 12, 12, 12),
            child: SingleChildScrollView(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  label('A 多脚本 (multiscript.txt)'),
                  body(corpus.multi),
                  const SizedBox(height: 6),
                  label('B emoji + 组合字符 (emoji.txt)'),
                  body(corpus.emoji),
                  const SizedBox(height: 6),
                  label('C 富文本 run (20/14/8/12pt)'),
                  SizedBox(
                    width: 560,
                    child: Text.rich(
                      TextSpan(
                        children: [
                          for (var i = 0; i < richLines.length; i++)
                            TextSpan(
                              text: '${richLines[i]}\n',
                              style: TextStyle(
                                fontSize: sizes[i % sizes.length],
                                color: colors[i % colors.length],
                                height: 1.2,
                              ),
                            ),
                        ],
                      ),
                    ),
                  ),
                  const SizedBox(height: 6),
                  label('D 多DPR同串 (12/15/24/36pt)'),
                  for (var i = 0; i < 4; i++)
                    Text(
                      '${[1, 1.25, 2, 3][i]}x: Hello 世界 مرحبا กข',
                      style: TextStyle(
                        fontSize: [12.0, 15.0, 24.0, 36.0][i],
                        color: kInk,
                        height: 1.1,
                      ),
                    ),
                ],
              ),
            ),
          ),
        ),
        const _StaticHud('text: static after first frame — no ticker'),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Scene 2: list scroll (1200x800, C3 semantics)
// ---------------------------------------------------------------------------

const int kItemCount = 1200;
const double kListWidth = 620;
const double kViewportH = 656;

double rowHeight(int i) {
  switch (i % 4) {
    case 0:
      return 44;
    case 1:
      return 92;
    case 2:
      return 60;
    default:
      return 76;
  }
}

class ListScene extends StatefulWidget {
  const ListScene({super.key});

  @override
  State<ListScene> createState() => _ListSceneState();
}

class _ListSceneState extends State<ListScene>
    with SingleTickerProviderStateMixin {
  final ScrollController _scroll = ScrollController();
  final ValueNotifier<String> _hud = ValueNotifier<String>('initializing');
  late final Ticker _ticker;
  Duration _last = Duration.zero;
  double _elapsed = 0;
  double _y = 0;
  double _maxY = 0;
  int _dir = 1;
  double _lastHud = -1;

  @override
  void initState() {
    super.initState();
    var total = 0.0;
    for (var i = 0; i < kItemCount; i++) {
      total += rowHeight(i);
    }
    _maxY = math.max(0.0, total - kViewportH).toDouble();
    _ticker = createTicker(_tick)..start();
  }

  void _tick(Duration now) {
    final dt = (now - _last).inMicroseconds / 1e6;
    _last = now;
    if (dt <= 0 || dt > 0.25) return;
    _elapsed += dt;
    final ct = _elapsed % 9.0;
    double speed;
    String phase;
    if (ct < 3) {
      phase = 'Steady';
      speed = 160;
    } else if (ct < 6) {
      phase = 'Spike';
      speed = 2600;
    } else {
      phase = 'Recover';
      speed = math.max(40.0, 2600 * (1 - (ct - 6) / 3));
    }
    _y += _dir * speed * dt;
    if (_y >= _maxY) {
      _y = _maxY;
      _dir = -1;
    } else if (_y <= 0) {
      _y = 0;
      _dir = 1;
    }
    if (_scroll.hasClients && (_scroll.offset - _y).abs() > 0.01) {
      _scroll.jumpTo(_y);
    }
    if (_elapsed - _lastHud >= 0.5) {
      _lastHud = _elapsed;
      final first = (_y / 68).floor();
      _hud.value = '$phase  y=${_y.toStringAsFixed(0)}  '
          'rows ~$first..${math.min(kItemCount, first + 16)} / $kItemCount';
    }
  }

  @override
  void dispose() {
    _ticker.dispose();
    _scroll.dispose();
    _hud.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _Header('Flutter ruler — list scroll (C3 semantic, 1200 variable rows)'),
        Expanded(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(12, 12, 12, 12),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                const SizedBox(width: 260, child: _Legend()),
                const SizedBox(width: 12),
                Expanded(
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      SizedBox(
                        width: kListWidth,
                        child: Scrollbar(
                          controller: _scroll,
                          thumbVisibility: true,
                          thickness: 6,
                          radius: const Radius.circular(3),
                          child: ListView.builder(
                            controller: _scroll,
                            physics: const NeverScrollableScrollPhysics(),
                            padding: EdgeInsets.zero,
                            scrollCacheExtent:
                                const ScrollCacheExtent.pixels(120),
                            itemCount: kItemCount,
                            itemBuilder: (context, i) =>
                                _ListRow(index: i),
                          ),
                        ),
                      ),
                      const SizedBox(width: 16),
                      const Expanded(child: _RightPanel()),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
        SizedBox(
          height: 72,
          child: _LiveHud(value: _hud),
        ),
      ],
    );
  }
}

class _ListRow extends StatelessWidget {
  const _ListRow({required this.index});

  final int index;

  @override
  Widget build(BuildContext context) {
    final h = rowHeight(index);
    final kind = index % 4;
    Widget child;
    switch (kind) {
      case 0:
        child = Padding(
          padding: const EdgeInsets.only(left: 14, top: 12),
          child: Text(
            'row ${index.toString().padLeft(4, '0')} · text line — C3 semantic',
            style: const TextStyle(fontSize: 15, color: kInk),
          ),
        );
        break;
      case 1:
        final base = Color.fromARGB(
          255,
          (40 + (index % 7) * 12).clamp(0, 255).toInt(),
          (89 + (index % 5) * 10).clamp(0, 255).toInt(),
          140,
        );
        child = Padding(
          padding: const EdgeInsets.only(left: 14, top: 10),
          child: Container(
            width: kListWidth - 28,
            height: h - 20,
            color: base,
            alignment: Alignment.centerLeft,
            child: Container(
              margin: const EdgeInsets.only(left: 14),
              width: 140,
              height: 24,
              color: const Color(0xFFF28C26),
            ),
          ),
        );
        break;
      case 2:
        child = Stack(
          children: [
            Positioned(
              left: 14,
              top: 8,
              child: Text(
                'row ${index.toString().padLeft(4, '0')} · mixed text + chip',
                style: const TextStyle(fontSize: 14, color: Color(0xFFB8D9BF)),
              ),
            ),
            Positioned(
              right: 14,
              top: 30,
              child: Container(
                width: 72,
                height: 18,
                color: Color.fromARGB(
                  255,
                  80 + (index % 6) * 15,
                  165,
                  90,
                ),
              ),
            ),
          ],
        );
        break;
      default:
        final src = index % 24;
        child = Stack(
          children: [
            Positioned(
              left: 14,
              top: 10,
              child: Image.asset(
                'assets/c3_src_${src.toString().padLeft(2, '0')}.png',
                width: 96,
                height: 56,
                fit: BoxFit.cover,
                gaplessPlayback: true,
              ),
            ),
            Positioned(
              left: 122,
              top: 28,
              child: Text(
                'row ${index.toString().padLeft(4, '0')} · image #$src',
                style: const TextStyle(fontSize: 13, color: Color(0xFFCCD2E6)),
              ),
            ),
          ],
        );
        break;
    }
    return SizedBox(height: h, child: child);
  }
}

class _RightPanel extends StatelessWidget {
  const _RightPanel();

  @override
  Widget build(BuildContext context) {
    const labels = [
      'STATIC DENSE',
      '不随滚动重绘',
      '4x4 nested cells',
      'range / image counters',
      'scroll thumb',
      'same content every frame',
      'no rebuild of static panel',
      'C3 semantic mirror',
    ];
    return Container(
      color: kPanel,
      padding: const EdgeInsets.all(12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text('STATIC DENSE',
              style: TextStyle(fontSize: 12, color: Color(0xFF8CBFF2))),
          const SizedBox(height: 8),
          for (var y = 0; y < 4; y++)
            Padding(
              padding: const EdgeInsets.only(bottom: 10),
              child: Row(
                children: [
                  for (var x = 0; x < 4; x++)
                    Container(
                      width: 48,
                      height: 48,
                      margin: const EdgeInsets.only(right: 10),
                      color: Color.fromARGB(
                        255,
                        (36 + (x + y) * 8).clamp(0, 255).toInt(),
                        (115 + (x + y) * 6).clamp(0, 255).toInt(),
                        (77 + (x * y) * 8).clamp(0, 255).toInt(),
                      ),
                    ),
                ],
              ),
            ),
          const SizedBox(height: 4),
          for (final s in labels.skip(1).take(7))
            Padding(
              padding: const EdgeInsets.only(bottom: 6),
              child: Text(s,
                  style: const TextStyle(fontSize: 12, color: kMuted)),
            ),
        ],
      ),
    );
  }
}

class _Legend extends StatelessWidget {
  const _Legend();

  @override
  Widget build(BuildContext context) {
    const rows = [
      ['C3 列表语义', '1200 项变高行'],
      ['行高', '44 / 92 / 60 / 76'],
      ['行型', '文本 / 色块 / chip / 图'],
      ['相位', 'Steady / Spike / Recover'],
      ['滚动', '160 → 2600 px/s'],
      ['缓存', '120px (2 行)'],
      ['静态区', '右侧 4x4 + legend'],
      ['对照臂', 'examples/ui_wr_c3_list_scroll'],
    ];
    return Container(
      color: kPanel,
      padding: const EdgeInsets.all(14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text('C3 SCROLL',
              style: TextStyle(fontSize: 14, color: Color(0xFF8CBFF2))),
          const SizedBox(height: 12),
          for (final r in rows)
            Padding(
              padding: const EdgeInsets.only(bottom: 12),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(r[0],
                      style: const TextStyle(fontSize: 11, color: kMuted)),
                  const SizedBox(height: 2),
                  Text(r[1], style: const TextStyle(fontSize: 13, color: kInk)),
                ],
              ),
            ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Scene 4: full-window repaint (1200x800, row 4 / R0 mirror)
// ---------------------------------------------------------------------------

/// Row 4 arm: every frame repaints >=90% of the 1200x800 client area. The
/// full-bleed animated gradient keeps no static band, mirroring R0's
/// full_paint full-tree repaint (damage ~1.0 every frame).
class FullScene extends StatefulWidget {
  const FullScene({super.key});

  @override
  State<FullScene> createState() => _FullSceneState();
}

class _FullSceneState extends State<FullScene>
    with SingleTickerProviderStateMixin {
  late final AnimationController _tick =
      AnimationController(vsync: this, duration: const Duration(seconds: 2))
        ..repeat();

  @override
  void dispose() {
    _tick.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _tick,
      builder: (context, _) => CustomPaint(
        size: MediaQuery.of(context).size,
        painter: _FullRepaintPainter(_tick.value),
      ),
    );
  }
}

class _FullRepaintPainter extends CustomPainter {
  const _FullRepaintPainter(this.t);

  final double t;

  @override
  void paint(Canvas canvas, Size size) {
    // Animated gradient filling the whole surface — no static band survives.
    final rect = Offset.zero & size;
    final grad = Paint()
      ..shader = LinearGradient(
        begin: Alignment(-1 + 2 * t, 0),
        end: Alignment(1 + 2 * t, 0),
        colors: [
          HSLColor.fromAHSL(1, (t * 90) % 360, 0.5, 0.35).toColor(),
          HSLColor.fromAHSL(1, (t * 90 + 120) % 360, 0.5, 0.25).toColor(),
          HSLColor.fromAHSL(1, (t * 90 + 240) % 360, 0.5, 0.15).toColor(),
        ],
      ).createShader(rect);
    canvas.drawRect(rect, grad);

    // Moving shapes on top keep every frame's pixel content distinct.
    final w = size.width;
    final h = size.height;
    for (var i = 0; i < 6; i++) {
      final p = (t + i * 0.166) % 1.0;
      final cx = w * p;
      final cy = h * (0.2 + 0.6 * (0.5 + 0.5 * math.sin(p * math.pi * 2)));
      canvas.drawRect(
        Rect.fromCenter(
          center: Offset(cx, cy),
          width: 90,
          height: 60,
        ),
        Paint()..color = Color.fromRGBO(255, 255, 255, 0.16),
      );
    }
    final bar = Paint()..color = const Color(0x66E2E7F2);
    canvas.drawRect(
      Rect.fromLTWH(0, h - 14, w * 0.15 + w * 0.7 * t % w, 8),
      bar,
    );
  }

  @override
  bool shouldRepaint(covariant _FullRepaintPainter old) =>
      old.t != t; // changes every frame by design.
}

// ---------------------------------------------------------------------------
// Scene 5: async decode grid (1200x800, row 5 / R10 mirror)
// ---------------------------------------------------------------------------

const int kGridCols = 4;
const int kGridRows = 5;
const int kGridTiles = kGridCols * kGridRows;

class GridScene extends StatefulWidget {
  const GridScene({super.key});

  @override
  State<GridScene> createState() => _GridSceneState();
}

class _GridSceneState extends State<GridScene>
    with SingleTickerProviderStateMixin {
  final List<ruler_ui.Image?> _imgs = List<ruler_ui.Image?>.filled(kGridTiles, null);
  late final AnimationController _hot =
      AnimationController(vsync: this, duration: const Duration(seconds: 2))
        ..repeat();

  @override
  void initState() {
    super.initState();
    // Decode all 20 real files concurrently (same arrival semantics as R10:
    // per-source goroutine, each cell lights up exactly when its decode done).
    for (var i = 0; i < kGridTiles; i++) {
      _decode(i);
    }
  }

  Future<void> _decode(int i) async {
    final data =
        await rootBundle.load('assets/grid/async_${i.toString().padLeft(2, '0')}.png');
    final codec = await ruler_ui.instantiateImageCodec(data.buffer.asUint8List());
    final frame = await codec.getNextFrame();
    if (!mounted) return;
    setState(() => _imgs[i] = frame.image);
  }

  @override
  void dispose() {
    _hot.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const _Header('Flutter ruler — async decode grid (R10 semantic)'),
        Expanded(
          child: LayoutBuilder(
            builder: (context, c) {
              return Padding(
                padding: const EdgeInsets.all(20),
                child: Stack(
                  children: [
                    Column(
                      children: [
                        for (var r = 0; r < kGridRows; r++)
                          Expanded(
                            child: Row(
                              children: [
                                for (var cc = 0; cc < kGridCols; cc++)
                                  Expanded(
                                    child: _TileCell(
                                      index: r * kGridCols + cc,
                                      img: _imgs[r * kGridCols + cc],
                                    ),
                                  ),
                              ],
                            ),
                          ),
                      ],
                    ),
                    Align(
                      alignment: const Alignment(0.92, 0.94),
                      child: RepaintBoundary(
                        child: AnimatedBuilder(
                          animation: _hot,
                          builder: (context, _) => Container(
                            width: 70,
                            height: 70,
                            color: Color.fromARGB(
                              255,
                              (150 + 100 * _hot.value).round(),
                              60,
                              60,
                            ),
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              );
            },
          ),
        ),
        const _StaticHud('grid: each tile its own RepaintBoundary; decodes light per tile'),
      ],
    );
  }
}

class _TileCell extends StatelessWidget {
  const _TileCell({required this.index, required this.img});

  final int index;
  final ruler_ui.Image? img;

  @override
  Widget build(BuildContext context) {
    return RepaintBoundary(
      child: Padding(
        padding: const EdgeInsets.all(6),
        child: Container(
          color: const Color(0xFF23283A),
          child: Padding(
            padding: const EdgeInsets.all(8),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('cell ${index.toString().padLeft(2, '0')}',
                    style: const TextStyle(fontSize: 11, color: kMuted)),
                Expanded(
                  child: Center(
                    // RULER_GRID_NOIMG=1 是隔离实验开关：占位不换图但解码照跑，
                    // 用于把「20 张纹理层的合成成本」和其余内容分开定位。
                    child: (img == null ||
                            Platform.environment['RULER_GRID_NOIMG'] == '1')
                        ? Container(color: const Color(0xFF14171F))
                        : RawImage(image: img, fit: BoxFit.contain),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Scene 6: boundary grid (1200x800, row 6 / R3 semantics mirror)
// ---------------------------------------------------------------------------

/// Row 6 arm: 4x4 grid of RepaintBoundary cells. Each cell has a nested inner
/// boundary with a hot spot; a rotating index repaints 1 cell + 1 inner spot
/// per beat while the other 15 cells stay cached (Replay semantics), mirroring
/// R3/R3b's boundary replay + dirty re-record mix at the same 16-cell scale.
class BoundaryScene extends StatefulWidget {
  const BoundaryScene({super.key});

  @override
  State<BoundaryScene> createState() => _BoundarySceneState();
}

class _BoundarySceneState extends State<BoundaryScene>
    with SingleTickerProviderStateMixin {
  late final Ticker _ticker;
  int _beat = 0;
  int _shownBeat = -1;
  Duration _last = Duration.zero;

  @override
  void initState() {
    super.initState();
    _ticker = createTicker(_tick)..start();
  }

  void _tick(Duration now) {
    final dt = (now - _last).inMicroseconds / 1e6;
    _last = now;
    if (dt <= 0 || dt > 0.25) return;
    _beat += (dt * 60).round(); // frame-beat counter at 60fps target
    // Rebuild only when the beat index actually moves (30 frames/beat at
    // 60fps): per-frame setState would churn all 16 cells' build+layout for
    // nothing — the ruler measures boundary replay/re-record cost, not
    // framework rebuild tax.
    final b = _beat ~/ 30;
    if (b != _shownBeat) {
      _shownBeat = b;
      setState(() {});
    }
  }

  @override
  void dispose() {
    _ticker.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const _Header('Flutter ruler — boundary grid (R3/R3b semantic)'),
        Expanded(
          child: GridView.count(
            crossAxisCount: 4,
            childAspectRatio: (300 / 170),
            children: [
              for (var i = 0; i < 16; i++)
                _BoundaryCell(index: i, beat: _beat ~/ 30),
            ],
          ),
        ),
        const _StaticHud('boundary: 1 cell + 1 inner spot repaint per beat, rest cached'),
      ],
    );
  }
}

class _BoundaryCell extends StatelessWidget {
  const _BoundaryCell({required this.index, required this.beat});

  final int index;
  final int beat;

  @override
  Widget build(BuildContext context) {
    final outerHot = (beat % 16) == index;
    final innerHot = ((beat ~/ 2) % 16) == index;
    return RepaintBoundary(
      child: Container(
        margin: const EdgeInsets.all(6),
        color: outerHot ? const Color(0xFF4A3B2E) : const Color(0xFF23283A),
        padding: const EdgeInsets.all(10),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('b${index.toString().padLeft(2, '0')}',
                style: const TextStyle(fontSize: 11, color: kMuted)),
            const SizedBox(height: 6),
            Expanded(
              child: Align(
                alignment: Alignment.topRight,
                child: RepaintBoundary(
                  child: Container(
                    width: 46,
                    height: 46,
                    color: innerHot
                        ? const Color(0xFFE0A34C)
                        : const Color(0xFF2E3548),
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Scene 8: overlay (1200x800, row 8 / R8 mirror)
// ---------------------------------------------------------------------------

/// Row 8 arm: base band keeps one per-frame hot box; a dimming barrier +
/// floating panel stack opens at t=1.2s (R8 gate timeline) and stays open, so
/// the measured window runs the overlay-on steady state with the same open
/// cost amortized outside the sampled window.
class OverlayScene extends StatefulWidget {
  const OverlayScene({super.key});

  @override
  State<OverlayScene> createState() => _OverlaySceneState();
}

class _OverlaySceneState extends State<OverlayScene>
    with SingleTickerProviderStateMixin {
  bool _open = false;
  late final AnimationController _hot =
      AnimationController(vsync: this, duration: const Duration(seconds: 2))
        ..repeat();

  @override
  void initState() {
    super.initState();
    Timer(const Duration(milliseconds: 1200), () {
      if (mounted) setState(() => _open = true);
    });
  }

  @override
  void dispose() {
    _hot.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final base = Stack(
      children: [
        GridView.count(
          crossAxisCount: 8,
          childAspectRatio: 2.0,
          children: [
            for (var i = 0; i < 32; i++)
              Container(
                margin: const EdgeInsets.all(6),
                color: i.isOdd ? const Color(0xFF2A3142) : const Color(0xFF1E2430),
              ),
          ],
        ),
        Align(
          alignment: const Alignment(0.86, 0.2),
          child: RepaintBoundary(
            child: AnimatedBuilder(
              animation: _hot,
              builder: (context, _) => Container(
                width: 70,
                height: 70,
                color: Color.fromARGB(
                  255,
                  (150 + 100 * _hot.value).round(),
                  60,
                  90,
                ),
              ),
            ),
          ),
        ),
      ],
    );

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const _Header('Flutter ruler — overlay stack (R8 semantic)'),
        Expanded(
          child: Stack(
            children: [
              Positioned.fill(child: base),
              if (_open) ...[
                // RULER_OVERLAY_NOBARRIER=1 是隔离实验开关：去掉全屏调光层，
                // 把「半透明全屏混合层」的成本单独定位。
                if (Platform.environment['RULER_OVERLAY_NOBARRIER'] != '1')
                  const Positioned.fill(
                    child: IgnorePointer(
                      child: ColoredBox(
                        color: Color(0x73000000),
                      ),
                    ),
                  ),
                Center(
                  child: RepaintBoundary(
                    child: Container(
                      width: 480,
                      height: 340,
                      color: const Color(0xFF2E3548),
                      padding: const EdgeInsets.all(24),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          const Text(
                            'OVERLAY PANEL',
                            style: TextStyle(
                              fontSize: 16,
                              color: kInk,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                          const SizedBox(height: 12),
                          const Text(
                            'R8 mirror: barrier + panel opened at 1.2s, held.',
                            style: TextStyle(fontSize: 12, color: kMuted),
                          ),
                          const Spacer(),
                          Align(
                            alignment: Alignment.bottomRight,
                            child: Container(
                              padding: const EdgeInsets.symmetric(
                                  horizontal: 16, vertical: 8),
                              color: const Color(0xFF3A4560),
                              child: const Text('OK',
                                  style: TextStyle(fontSize: 13, color: kInk)),
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              ],
            ],
          ),
        ),
        const _StaticHud('overlay: opened at 1.2s and held; only base HOT ticks per frame'),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Scene 7: resize responsive layout (1200x800, row 7 / R11 mirror)
// ---------------------------------------------------------------------------

/// Row 7 arm: window resize waves are delivered by the runner via
/// RULER_ACTIONS (e.g. "r1@8:1000x700;r2@16:1200x800" — scheduled inside the
/// measured window). The content intentionally reflows on every size pulse
/// (responsive row heights + rewrapped text), so each wave invalidates the
/// full layout like R11's size/DPR cache waves.
class ResizeScene extends StatelessWidget {
  const ResizeScene({super.key});

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const _Header('Flutter ruler — resize waves (R11 semantic)'),
        Expanded(
          child: Padding(
            padding: const EdgeInsets.all(12),
            child: LayoutBuilder(
              builder: (context, c) {
                final w = c.maxWidth;
                final h = c.maxHeight;
                final boxH = h * 0.30;
                return Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    SizedBox(
                      height: boxH,
                      child: Row(
                        children: [
                          for (var i = 0; i < 6; i++)
                            Expanded(
                              child: Container(
                                margin: const EdgeInsets.all(6),
                                color: Color.fromARGB(
                                  255,
                                  40 + i * 18,
                                  60,
                                  90 + i * 14,
                                ),
                              ),
                            ),
                        ],
                      ),
                    ),
                    const SizedBox(height: 8),
                    Expanded(
                      child: Row(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          SizedBox(
                            width: w * 0.62,
                            child: SingleChildScrollView(
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  for (var i = 0; i < 8; i++)
                                    Padding(
                                      padding: const EdgeInsets.only(bottom: 4),
                                      child: Text(
                                        'resize reflow block $i — '
                                        'width ${w.toStringAsFixed(0)} drives '
                                        'full re-layout + re-wrap of this text '
                                        'layer; each wave invalidates caches.',
                                        style: const TextStyle(
                                            fontSize: 13, color: kInk),
                                      ),
                                    ),
                                ],
                              ),
                            ),
                          ),
                          const SizedBox(width: 8),
                          Expanded(
                            child: GridView.count(
                              crossAxisCount: 4,
                              children: [
                                for (var i = 0; i < 16; i++)
                                  Container(
                                    margin: const EdgeInsets.all(4),
                                    color: const Color(0xFF23283A),
                                  ),
                              ],
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                );
              },
            ),
          ),
        ),
        const _StaticHud('resize: waves arrive via RULER_ACTIONS (runner GTK resize)'),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Scene 9: FG/BG (1200x800, row 9 / R17 mirror)
// ---------------------------------------------------------------------------

/// Row 9 arm: same content scale as R17 — an FG panel whose progress bar and
/// frame counter advance every frame plus a static 4x4 color grid and 8
/// labels. The runner minimizes the window via RULER_ACTIONS at t=22 (R17's
/// FG→BG boundary); Flutter/Impeller will keep consuming compositor callbacks
/// while hidden, which is exactly what the ruler wants to expose next to
/// gpui's zero-submit latch.
class MinScene extends StatefulWidget {
  const MinScene({super.key});

  @override
  State<MinScene> createState() => _MinSceneState();
}

class _MinSceneState extends State<MinScene>
    with SingleTickerProviderStateMixin {
  late final AnimationController _spin =
      AnimationController(vsync: this, duration: const Duration(seconds: 2))
        ..repeat();

  int get _fgTick => (_spin.value * 99).round();

  @override
  void dispose() {
    _spin.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    Widget label(String s) => Padding(
          padding: const EdgeInsets.all(4),
          child: Text('label $s',
              style: const TextStyle(fontSize: 12, color: kMuted)),
        );

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const _Header('Flutter ruler — FG/BG minimize (R17 semantic)'),
        Expanded(
          child: Stack(
            children: [
              Align(
                alignment: Alignment.topRight,
                child: SizedBox(
                  width: 420,
                  child: GridView.count(
                    crossAxisCount: 4,
                    childAspectRatio: 1.0,
                    children: [
                      for (var i = 0; i < 16; i++)
                        Container(
                          margin: const EdgeInsets.all(6),
                          color: Color.fromARGB(
                            255,
                            30 + (i * 7) % 90,
                            45 + (i * 5) % 80,
                            80 + (i * 11) % 120,
                          ),
                        ),
                    ],
                  ),
                ),
              ),
              Align(
                alignment: Alignment.topLeft,
                child: Padding(
                  padding: const EdgeInsets.all(24),
                  child: RepaintBoundary(
                    child: AnimatedBuilder(
                      animation: _spin,
                      builder: (context, _) => Container(
                        width: 260,
                        height: 150,
                        color: const Color(0xFF2A5550),
                        padding: const EdgeInsets.all(10),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text('FG frame $_fgTick',
                                style: const TextStyle(
                                    fontSize: 13, color: kInk)),
                            const SizedBox(height: 10),
                            Container(
                              height: 24,
                              color: const Color(0xFF16211E),
                              child: FractionallySizedBox(
                                alignment: Alignment.centerLeft,
                                widthFactor: (_spin.value % 1.0),
                                child: Container(
                                    color: const Color(0xFF4CD98C)),
                              ),
                            ),
                            const SizedBox(height: 10),
                            const Text('前台流动 / 后台冻结',
                                style: TextStyle(
                                    fontSize: 12, color: Color(0xFFBFE0C6))),
                          ],
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            ],
          ),
        ),
        Wrap(
          children: [for (var i = 0; i < 8; i++) label('$i')],
        ),
        const _StaticHud('min: runner minimizes @t=22 (RULER_ACTIONS=minimize:22)'),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Scene 13: vector density via CustomPaint (1200x800, row 13 arm)
// ---------------------------------------------------------------------------

/// Row 13 arm for the Flutter ruler: 100 animated cubic bezier strokes at
/// 60fps, mirroring `ui_base_batch_cap` vector mode's 100-curve steady band
/// (300/600 curves are capacity probes, not the comparison band).
class CPScene extends StatefulWidget {
  const CPScene({super.key});

  @override
  State<CPScene> createState() => _CPSceneState();
}

class _CPSceneState extends State<CPScene>
    with SingleTickerProviderStateMixin {
  late final AnimationController _tick =
      AnimationController(vsync: this, duration: const Duration(seconds: 3))
        ..repeat();

  @override
  void dispose() {
    _tick.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _tick,
      builder: (context, _) => CustomPaint(
        size: MediaQuery.of(context).size,
        painter: _VectorPainter(_tick.value),
      ),
    );
  }
}

class _VectorPainter extends CustomPainter {
  const _VectorPainter(this.t);

  final double t;

  @override
  void paint(Canvas canvas, Size size) {
    final w = size.width;
    final h = size.height;
    for (var i = 0; i < 100; i++) {
      final p = (t + i / 100.0) % 1.0;
      final y0 = h * (i % 10 + 1) / 11.0;
      final path = Path()
        ..moveTo(12, y0)
        ..cubicTo(
          w * 0.30,
          y0 + (p - 0.5) * h * 0.18,
          w * 0.70,
          y0 - (p - 0.5) * h * 0.18,
          w - 12,
          y0 + ((i % 3) - 1) * h * 0.02,
        );
      canvas.drawPath(
        path,
        Paint()
          ..style = PaintingStyle.stroke
          ..strokeWidth = 2
          ..color = HSLColor.fromAHSL(1, (p * 360) % 360, 0.6, 0.55).toColor(),
      );
    }
  }

  @override
  bool shouldRepaint(covariant _VectorPainter old) => old.t != t;
}

class _LiveHud extends StatelessWidget {
  const _LiveHud({required this.value});

  final ValueListenable<String> value;

  @override
  Widget build(BuildContext context) {
    return Container(
      color: const Color(0xFF11151D),
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
      child: Row(
        children: [
          const Text('C3 / Flutter',
              style: TextStyle(fontSize: 13, color: Color(0xFF8CBFF2))),
          const SizedBox(width: 16),
          Expanded(
            child: ValueListenableBuilder<String>(
              valueListenable: value,
              builder: (context, v, _) => Text(
                v,
                style: const TextStyle(fontSize: 12, color: kMuted),
              ),
            ),
          ),
          const Text('60fps target · /proc CPU sampled externally',
              style: TextStyle(fontSize: 11, color: Color(0xFF6E7A91))),
        ],
      ),
    );
  }
}

class _Header extends StatelessWidget {
  const _Header(this.title);

  final String title;

  @override
  Widget build(BuildContext context) {
    return Container(
      height: 48,
      color: const Color(0xFF151922),
      padding: const EdgeInsets.symmetric(horizontal: 14),
      alignment: Alignment.centerLeft,
      child: Row(
        children: [
          Text(title,
              style: const TextStyle(
                  fontSize: 14,
                  color: kInk,
                  fontWeight: FontWeight.w500)),
          const Spacer(),
          Text('P3 Flutter ruler · RULER_SCENE=$kScene',
              style: const TextStyle(fontSize: 11, color: kMuted)),
        ],
      ),
    );
  }
}

class _StaticHud extends StatelessWidget {
  const _StaticHud(this.text);

  final String text;

  @override
  Widget build(BuildContext context) {
    return Container(
      height: 72,
      color: const Color(0xFF11151D),
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
      alignment: Alignment.centerLeft,
      child: Row(
        children: [
          const Text('M5 / Flutter',
              style: TextStyle(fontSize: 13, color: Color(0xFF8CBFF2))),
          const SizedBox(width: 16),
          Text(text, style: const TextStyle(fontSize: 12, color: kMuted)),
        ],
      ),
    );
  }
}
