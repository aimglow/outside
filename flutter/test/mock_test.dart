// mock だけが持つ振る舞い (渡した値だけで決まる・呼ばれた内容が読める)。
import 'dart:convert';
import 'dart:typed_data';

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

  group('TC-OUT-IDGEN-MOCK', () {
    test('id-0001 から 呼んだ順に決まった ID を返し mock ごとに 1 から始まる', () {
      final first = MockIdGen();
      expect(
        [first.newId(), first.newId(), first.newId()],
        ['id-0001', 'id-0002', 'id-0003'],
      );
      final second = MockIdGen();
      expect(second.newId(), 'id-0001');
      expect(first.newId(), 'id-0004');
    });

    test('呼ばれた回数が読める', () {
      final idGen = MockIdGen();
      expect(idGen.calls, 0);
      idGen.newId();
      idGen.newId();
      expect(idGen.calls, 2);
    });
  });

  group('TC-OUT-RANDOM-FE MockRandom', () {
    List<int> read(MockRandom random, int n) {
      final buf = Uint8List(n);
      random.read(buf);
      return buf;
    }

    test('同じ種・同じ呼び順なら 同じ byte', () {
      final a = MockRandom.text('seed-a');
      final b = MockRandom.text('seed-a');
      expect([read(a, 16), read(a, 8)], [read(b, 16), read(b, 8)]);
      expect(read(MockRandom([1, 2, 3]), 16), read(MockRandom([1, 2, 3]), 16));
    });

    test('文字列の種は その UTF-8 の byte 列の種と同じ列', () {
      expect(
        read(MockRandom.text('種'), 16),
        read(MockRandom(utf8.encode('種')), 16),
      );
    });

    test('違う種は 違う byte', () {
      expect(
        read(MockRandom.text('seed-a'), 24),
        isNot(equals(read(MockRandom.text('seed-b'), 24))),
      );
      expect(
        read(MockRandom(const []), 24),
        isNot(equals(read(MockRandom(const [0]), 24))),
      );
    });

    test('分けて読んでも 続けて読んでも 同じ列', () {
      final whole = read(MockRandom.text('stream'), 24);
      final split = MockRandom.text('stream');
      expect([...read(split, 16), ...read(split, 8)], whole);
      final odd = MockRandom.text('stream');
      expect([
        ...read(odd, 3),
        ...read(odd, 0),
        ...read(odd, 5),
        ...read(odd, 16),
      ], whole);
    });

    test(
      '列は FNV-1a 64bit の種から始まる splitmix64 (next・angular の createMockRandom と同じ列)',
      () {
        expect(read(MockRandom.text('outside'), 12), [
          23,
          1,
          114,
          187,
          19,
          129,
          210,
          158,
          59,
          154,
          56,
          168,
        ]);
        expect(read(MockRandom.text(''), 4), [48, 255, 164, 107]);
      },
    );

    test('読んだ長さを 読んだ順に記録する', () {
      final random = MockRandom.text('calls');
      read(random, 16);
      read(random, 8);
      read(random, 0);
      expect(random.calls, [16, 8, 0]);
    });

    test('withErr の後は その誤りを投げ 記録は残す。withErr(null) で 列の続きに戻る', () {
      final whole = read(MockRandom.text('err'), 16);
      final random = MockRandom.text('err');
      final first = read(random, 8);
      final err = StateError('entropy exhausted');
      expect(random.withErr(err), same(random));
      expect(() => read(random, 4), throwsA(same(err)));
      expect(random.withErr(null), same(random));
      expect([...first, ...read(random, 8)], whole);
      expect(random.calls, [8, 4, 8]);
    });
  });
}
