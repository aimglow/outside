// 契約を 本物と mock の両方に掛ける。本物は test が作った物 (loopback の HttpServer 等) だけを触る。
import 'dart:async';
import 'dart:io' as io;

import 'package:outside/mock.dart';
import 'package:outside/outside.dart';
import 'package:test/test.dart';

import 'contract.dart';

/// RealStorage に渡す memory の上の実体。
class _MemoryBackend implements StorageBackend {
  final Map<String, String> _values = {};

  @override
  Future<String?> get(String key) async => _values[key];

  @override
  Future<void> set(String key, String value) async => _values[key] = value;

  @override
  Future<void> remove(String key) async => _values.remove(key);
}

/// 決めに従って答える loopback の server を起こし RealHttp を向ける。
/// 届いた内容を sent に積む。failures の宛先は 応答を返さずに接続を切る。
Future<HttpUnderTest> _realHttp(HttpSetup setup) async {
  final server = await io.HttpServer.bind(io.InternetAddress.loopbackIPv4, 0);
  final client = io.HttpClient();
  addTearDown(() async {
    client.close(force: true);
    await server.close(force: true);
  });
  final base = Uri.parse('http://${server.address.address}:${server.port}/');
  final sent = <HttpRequest>[];
  server.listen((req) async {
    final headers = <String, String>{};
    req.headers.forEach((name, values) => headers[name] = values.join(', '));
    final body = await req.fold<List<int>>(<int>[], (b, c) => b..addAll(c));
    sent.add(
      HttpRequest(
        req.method,
        base.resolve(req.uri.toString()),
        headers: headers,
        bodyBytes: body,
      ),
    );
    final key = '${req.method} ${req.uri}';
    if (setup.failures.contains(key)) {
      final socket = await req.response.detachSocket(writeHeaders: false);
      socket.destroy();
      return;
    }
    final res = setup.responses[key];
    if (res == null) {
      req.response.statusCode = 599;
      await req.response.close();
      return;
    }
    req.response.statusCode = res.status;
    res.headers.forEach(req.response.headers.set);
    req.response.add(res.bodyBytes);
    await req.response.close();
  });
  return (http: RealHttp(client: client), base: base, sent: () => sent);
}

void main() {
  clockContract('real', () {
    final clock = RealClock();
    // 0 の長さの timer は 掛けた順に呼ばれるので 後から掛けた 0 の待ちは 先の timer の後に戻る。
    return (clock: clock, elapse: () => Future<void>.delayed(Duration.zero));
  });
  clockContract('mock', () {
    final clock = MockClock(DateTime.utc(2026, 1, 2, 3, 4, 5));
    return (clock: clock, elapse: () async => clock.advance(Duration.zero));
  });

  httpContract('real', _realHttp);
  httpContract('mock', (setup) async {
    final base = Uri.parse('https://example.test/');
    final mock = MockHttp();
    for (final e in setup.responses.entries) {
      final i = e.key.indexOf(' ');
      mock.respond(
        e.key.substring(0, i),
        base.resolve(e.key.substring(i + 1)),
        e.value,
      );
    }
    for (final key in setup.failures) {
      final i = key.indexOf(' ');
      mock.fail(
        key.substring(0, i),
        base.resolve(key.substring(i + 1)),
        'refused',
      );
    }
    return (http: mock, base: base, sent: () => mock.requests);
  });

  storageContract('real', () => RealStorage(_MemoryBackend()));
  storageContract('mock', MockStorage.new);

  configContract(
    'real・build 時の表',
    (table) => RealConfig(buildTime: table, runtime: const {}),
  );
  configContract('real・実行時の表', (table) => RealConfig(runtime: table));
  configContract('mock', MockConfig.new);

  idGenContract('real', RealIdGen.new);
  idGenContract('mock', MockIdGen.new);

  randomContract('real', RealRandom.new);
  randomContract('mock', () => MockRandom.text('contract'));
}
