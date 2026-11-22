// 契約を 本物と mock の両方に掛ける。本物は test が渡した物だけを触る。
import 'dart:async';

import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:outside/mock.dart';
import 'package:outside/outside.dart';

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

/// 決めに従って答え 届いた内容を sent に積む client。通信はしない。
http.Client _clientFor(HttpSetup setup, List<HttpRequest> sent) => MockClient((
  req,
) async {
  sent.add(
    HttpRequest(
      req.method,
      req.url,
      headers: req.headers,
      bodyBytes: req.bodyBytes,
    ),
  );
  final key = '${req.method} ${req.url}';
  final failure = setup.failures[key];
  if (failure != null) throw http.ClientException(failure, req.url);
  final res = setup.responses[key]!;
  return http.Response.bytes(res.bodyBytes, res.status, headers: res.headers);
});

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

  httpContract('real', (setup) {
    final sent = <HttpRequest>[];
    return (http: RealHttp(client: _clientFor(setup, sent)), sent: () => sent);
  });
  httpContract('mock', (setup) {
    final mock = MockHttp();
    for (final e in setup.responses.entries) {
      final i = e.key.indexOf(' ');
      mock.respond(
        e.key.substring(0, i),
        Uri.parse(e.key.substring(i + 1)),
        e.value,
      );
    }
    for (final e in setup.failures.entries) {
      final i = e.key.indexOf(' ');
      mock.fail(
        e.key.substring(0, i),
        Uri.parse(e.key.substring(i + 1)),
        e.value,
      );
    }
    return (http: mock, sent: () => mock.requests);
  });

  storageContract('real', () => RealStorage(_MemoryBackend()));
  storageContract('mock', MockStorage.new);

  configContract(
    'real・build 時の表',
    (table) => RealConfig(buildTime: table, runtime: const {}),
  );
  configContract('real・実行時の表', (table) => RealConfig(runtime: table));
  configContract('mock', MockConfig.new);
}
