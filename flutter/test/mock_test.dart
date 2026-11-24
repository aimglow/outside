// mock だけが持つ振る舞い (渡した値だけで決まる・呼ばれた内容が読める)。
import 'package:outside/mock.dart';
import 'package:outside/outside.dart';
import 'package:test/test.dart';

void main() {
  final start = DateTime.utc(2026, 1, 2, 3, 4, 5);

  group('TC-OUT-CLOCK-MOCK', () {
    test('決めた時刻を返し advance した分だけ進む', () {
      final clock = MockClock(start);
      expect(clock.now(), start);
      expect(clock.now(), start);
      clock.advance(const Duration(days: 400, milliseconds: 7));
      expect(
        clock.now(),
        start.add(const Duration(days: 400, milliseconds: 7)),
      );
      expect(clock.nowCalls, 3);
    });

    test('timer は 期限に達した advance で 期限の順に呼ばれ その時の時刻が見える', () {
      final clock = MockClock(start);
      final seen = <String>[];
      void note(String name) =>
          seen.add('$name@${clock.now().difference(start).inSeconds}');
      clock.setTimeout(const Duration(seconds: 30), () => note('late'));
      clock.setTimeout(const Duration(seconds: 10), () => note('early'));
      clock.advance(const Duration(seconds: 9));
      expect(seen, isEmpty);
      clock.advance(const Duration(hours: 1));
      expect(seen, ['early@10', 'late@30']);
      expect(clock.now(), start.add(const Duration(hours: 1, seconds: 9)));
      expect(clock.pendingTimeouts, 0);
    });

    test('callback の中で掛けた timer も 進めた範囲に入れば呼ばれる', () {
      final clock = MockClock(start);
      final seen = <int>[];
      clock.setTimeout(const Duration(seconds: 1), () {
        seen.add(1);
        clock.setTimeout(const Duration(seconds: 1), () => seen.add(2));
        clock.setTimeout(const Duration(seconds: 5), () => seen.add(3));
      });
      clock.advance(const Duration(seconds: 3));
      expect(seen, [1, 2]);
      expect(clock.pendingTimeouts, 1);
    });

    test('掛けた delay と 取り消した札が読める', () {
      final clock = MockClock(start);
      final h = clock.setTimeout(const Duration(seconds: 2), () {});
      clock.setTimeout(const Duration(seconds: -1), () {});
      clock.clearTimeout(h);
      expect(clock.scheduled, [
        const Duration(seconds: 2),
        const Duration(seconds: -1),
      ]);
      expect(clock.cleared, [h]);
      expect(clock.pendingTimeouts, 1);
    });

    test('負の長さの advance は 誤り', () {
      final clock = MockClock(start);
      expect(
        () => clock.advance(const Duration(seconds: -1)),
        throwsArgumentError,
      );
      expect(clock.now(), start);
    });
  });

  group('REQ-OUT-MOCK Http', () {
    final url = Uri.parse('https://example.test/a');

    test('送られた内容を 呼ばれた順に記録する', () async {
      final mock = MockHttp()
        ..respond('GET', url, HttpResponse(200, body: 'one'))
        ..respond('POST', url, HttpResponse(202));
      expect((await mock.request(HttpRequest('GET', url))).body, 'one');
      expect(
        (await mock.request(HttpRequest('POST', url, body: 'b'))).status,
        202,
      );
      expect(mock.requests.map((r) => '${r.method} ${r.url} ${r.body}'), [
        'GET $url ',
        'POST $url b',
      ]);
    });

    test('応答を決めていない宛先は 宛先を名乗る誤り (通信しない)', () async {
      final mock = MockHttp();
      await expectLater(
        mock.request(HttpRequest('GET', url)),
        throwsA(
          isA<StateError>().having(
            (e) => e.message,
            'message',
            contains('GET $url'),
          ),
        ),
      );
      expect(mock.requests, hasLength(1));
    });
  });

  group('REQ-OUT-MOCK Storage', () {
    test('空から始まり 呼ばれた内容と 今の中身が読める', () async {
      final mock = MockStorage();
      expect(mock.values, isEmpty);
      await mock.set('k', 'v');
      await mock.get('k');
      await mock.remove('k');
      expect(mock.calls, [
        (op: 'set', key: 'k', value: 'v'),
        (op: 'get', key: 'k', value: null),
        (op: 'remove', key: 'k', value: null),
      ]);
      expect(mock.values, isEmpty);
    });
  });

  group('REQ-OUT-MOCK Config', () {
    test('渡した表だけを返し 機体の環境変数は現れない。引かれた名前が読める', () {
      final mock = MockConfig({'API_URL': 'https://example.test'});
      expect(mock.get('API_URL'), 'https://example.test');
      // PATH と HOME は 機体には在るが 表には無い。
      expect(mock.get('PATH'), isNull);
      expect(mock.get('HOME'), isNull);
      expect(mock.lookups, ['API_URL', 'PATH', 'HOME']);
    });

    test('表を渡さなければ 何も返さない', () {
      expect(MockConfig().get('API_URL'), isNull);
    });
  });
}
