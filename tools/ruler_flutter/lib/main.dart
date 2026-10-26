// P3 Flutter comparison ruler.
//
// Three scenes, selected with RULER_SCENE:
//   spin  — 480x320, one 64px rotating icon (matrix row 2 / spin_min)
//   list  — 1200x800, 1200 variable-height rows + static panels (matrix row 3)
//   text  — 1200x800, multiscript/emoji/rich/multi-size dense page (row 12)
//
// The process prints machine-readable RULER_METRIC lines. CPU/RSS are sampled
// externally from /proc (see scripts/sample.sh), matching the gpui ruler.

import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:math' as math;
import 'dart:ui' show FramePhase;

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
