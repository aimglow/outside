// 本物だけが持つ振る舞い。test が渡した物だけを触る。
import 'dart:async';
import 'dart:io' as io;

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

  test('REQ-OUT-REAL-TESTED-HERE RealConfig は 渡した値の表 → 環境変数の順に引く (同じ名前なら 渡した値が勝つ)', () {
    final c = RealConfig(
      buildTime: const {'A': 'build', 'B': 'build'},
      runtime: const {'A': 'run'},
    );
    expect(c.get('A'), 'build');
    expect(c.get('B'), 'build');
    expect(RealConfig(runtime: const {'C': 'run'}).get('C'), 'run');
  });

  test('REQ-OUT-INJECT OutsideSet は 渡した口を そのまま持つ', () {
    final clock = RealClock();
    final config = RealConfig(runtime: const {});
    final idGen = RealIdGen();
    final set = OutsideSet(
      clock: clock,
      http: RealHttp(),
      storage: RealStorage(_NoBackend()),
      config: config,
      idGen: idGen,
    );
    expect(set.clock, same(clock));
    expect(set.config, same(config));
    expect(set.idGen, same(idGen));
  });

  test('REQ-OUT-INJECT OutsideSet.real は 5 つの本物の口を作る', () {
    final set = OutsideSet.real(storageBackend: _NoBackend());
    expect(set.clock, isA<RealClock>());
    expect(set.http, isA<RealHttp>());
    expect(set.storage, isA<RealStorage>());
    expect(set.config, isA<RealConfig>());
    expect(set.idGen, isA<RealIdGen>());
  });

  test('REQ-OUT-REAL-TESTED-HERE RealIdGen は UUID v4 の形の ID を返す', () {
    final uuidV4 = RegExp(
      r'^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$',
    );
    final idGen = RealIdGen();
    for (var i = 0; i < 50; i++) {
      expect(idGen.newId(), matches(uuidV4));
    }
  });

  group('REQ-OUT-REAL-TESTED-HERE RealHttp (test が起こした loopback の server)', () {
    late io.HttpServer server;
    late io.HttpClient client;
    late Uri base;

    setUp(() async {
      server = await io.HttpServer.bind(io.InternetAddress.loopbackIPv4, 0);
      client = io.HttpClient();
      base = Uri.parse('http://${server.address.address}:${server.port}/');
    });

    tearDown(() async {
      client.close(force: true);
      await server.close(force: true);
    });

    test('同じ名前の header が複数届くと ", " で繋いだ 1 つの値になる', () async {
      server.listen((req) async {
        req.response.headers
          ..add('x-many', 'a')
          ..add('x-many', 'b');
        await req.response.close();
      });
      final res = await RealHttp(
        client: client,
      ).request(HttpRequest('GET', base.resolve('/h')));
      expect(res.status, 200);
      expect(res.headers['x-many'], 'a, b');
    });

    test('bytes の body を そのまま送り そのまま受け取る', () async {
      final received = Completer<List<int>>();
      server.listen((req) async {
        final body = await req.fold<List<int>>(<int>[], (b, c) => b..addAll(c));
        received.complete(body);
        req.response.add([0, 255, 128]);
        await req.response.close();
      });
      final res = await RealHttp(client: client).request(
        HttpRequest('POST', base.resolve('/b'), bodyBytes: [1, 2, 250]),
      );
      expect(await received.future, [1, 2, 250]);
      expect(res.bodyBytes, [0, 255, 128]);
    });

    test('閉じた port への接続は HttpFailure', () async {
      final closed = await io.HttpServer.bind(
        io.InternetAddress.loopbackIPv4,
        0,
      );
      final url = Uri.parse('http://127.0.0.1:${closed.port}/');
      await closed.close(force: true);
      await expectLater(
        RealHttp(client: client).request(HttpRequest('GET', url)),
        throwsA(isA<HttpFailure>()),
      );
    });
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
