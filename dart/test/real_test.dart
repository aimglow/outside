// 本物だけが持つ振る舞い。test が渡した物だけを触る。
import 'dart:async';

import 'package:outside/outside.dart';
import 'package:test/test.dart';

/// zone が RealClock へ返す timer。実時間では発火しない。
class _HeldTimer implements Timer {
  _HeldTimer(this.delay, this.fire);

  final Duration delay;
  final void Function() fire;
  bool cancelled = false;

  @override
  void cancel() => cancelled = true;

  @override
  bool get isActive => !cancelled;

  @override
  int get tick => 0;
}

void main() {
  test('REQ-OUT-REAL-TESTED-HERE RealClock は delay を timer へ渡し 取り消しを伝える', () {
    final held = <_HeldTimer>[];
    runZoned(
      () {
        final clock = RealClock();
        var fired = 0;
        clock.setTimeout(const Duration(minutes: 5), () => fired++);
        final second = clock.setTimeout(
          const Duration(hours: 1),
          () => fired++,
        );
        expect(held.map((t) => t.delay), [
          const Duration(minutes: 5),
          const Duration(hours: 1),
        ]);
        clock.clearTimeout(second);
        expect(held[1].cancelled, isTrue);
        expect(held[0].cancelled, isFalse);
        held[0].fire();
        expect(fired, 1);
      },
      zoneSpecification: ZoneSpecification(
        createTimer: (self, parent, zone, duration, f) {
          final t = _HeldTimer(duration, f);
          held.add(t);
          return t;
        },
      ),
    );
  });

  test('REQ-OUT-REAL-TESTED-HERE RealConfig は 同じ名前なら 実行時の値が勝つ', () {
    final c = RealConfig(
      buildTime: const {'A': 'build', 'B': 'build'},
      runtime: const {'A': 'run'},
    );
    expect(c.get('A'), 'run');
    expect(c.get('B'), 'build');
  });

  test('REQ-OUT-INJECT OutsideSet は 渡した口を そのまま持つ', () {
    final clock = RealClock();
    final config = RealConfig(runtime: const {});
    final set = OutsideSet(
      clock: clock,
      http: RealHttp(),
      storage: RealStorage(_NoBackend()),
      config: config,
    );
    expect(set.clock, same(clock));
    expect(set.config, same(config));
  });
}

/// 呼ばれない実体。
class _NoBackend implements StorageBackend {
  @override
  Future<String?> get(String key) => throw UnimplementedError();

  @override
  Future<void> set(String key, String value) => throw UnimplementedError();

  @override
  Future<void> remove(String key) => throw UnimplementedError();
}
