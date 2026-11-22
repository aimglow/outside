// 口ごとの契約。本物と mock の両方に 同じ関数を掛ける。
import 'package:outside/outside.dart';
import 'package:test/test.dart';

/// Clock の契約。elapse は 0 の長さだけ時を進める手段 (本物 = event loop を 1 周、mock = advance)。
void clockContract(
  String name,
  ({Clock clock, Future<void> Function() elapse}) Function() create,
) {
  group('TC-OUT-CONTRACT Clock ($name)', () {
    test('now は UTC で 戻らない', () {
      final clock = create().clock;
      final a = clock.now();
      final b = clock.now();
      expect(a.isUtc, isTrue);
      expect(b.isBefore(a), isFalse);
    });

    test('setTimeout は その場では呼ばず 時が進むと 1 回呼ぶ', () async {
      final h = create();
      var fired = 0;
      h.clock.setTimeout(Duration.zero, () => fired++);
      expect(fired, 0);
      await h.elapse();
      expect(fired, 1);
      await h.elapse();
      expect(fired, 1);
    });

    test('同じ期限の timer は 掛けた順に呼ぶ', () async {
      final h = create();
      final order = <String>[];
      h.clock.setTimeout(Duration.zero, () => order.add('a'));
      h.clock.setTimeout(Duration.zero, () => order.add('b'));
      await h.elapse();
      expect(order, ['a', 'b']);
    });

    test('clearTimeout した timer は 呼ばれない。済んだ札の取り消しは 何もしない', () async {
      final h = create();
      var fired = 0;
      final cancelled = h.clock.setTimeout(Duration.zero, () => fired += 10);
      final kept = h.clock.setTimeout(Duration.zero, () => fired += 1);
      h.clock.clearTimeout(cancelled);
      h.clock.clearTimeout(cancelled);
      await h.elapse();
      expect(fired, 1);
      h.clock.clearTimeout(kept);
      await h.elapse();
      expect(fired, 1);
    });
  });
}

/// Http の契約に渡す応答の決め。key は「METHOD url」。
typedef HttpSetup = ({
  Map<String, HttpResponse> responses,
  Map<String, String> failures,
});

/// Http の契約。create は 決めを受けて Http と 送られた内容の読み口を返す。
void httpContract(
  String name,
  ({Http http, List<HttpRequest> Function() sent}) Function(HttpSetup) create,
) {
  final ok = Uri.parse('https://example.test/items?page=2');
  final missing = Uri.parse('https://example.test/missing');
  final down = Uri.parse('https://example.test/down');

  group('TC-OUT-CONTRACT Http ($name)', () {
    test('決めた応答 (status・header・body) が返る', () async {
      final h = create((
        responses: {
          'POST $ok': HttpResponse(
            201,
            headers: {'Content-Type': 'application/json'},
            body: '{"名前":"値"}',
          ),
        },
        failures: {},
      ));
      final res = await h.http.request(HttpRequest('post', ok, body: 'x'));
      expect(res.status, 201);
      expect(res.headers['content-type'], 'application/json');
      expect(res.body, '{"名前":"値"}');
    });

    test('送った method・url・header・body が そのまま届く', () async {
      final h = create((
        responses: {'PUT $ok': HttpResponse(204)},
        failures: {},
      ));
      await h.http.request(
        HttpRequest('put', ok, headers: {'x-token': 't1'}, body: '送る中身'),
      );
      final sent = h.sent();
      expect(sent, hasLength(1));
      expect(sent.single.method, 'PUT');
      expect(sent.single.url, ok);
      expect(sent.single.headers['x-token'], 't1');
      expect(sent.single.body, '送る中身');
    });

    test('2xx でない status は 誤りにせず 応答として返る', () async {
      final h = create((
        responses: {'GET $missing': HttpResponse(404, body: 'none')},
        failures: {},
      ));
      final res = await h.http.request(HttpRequest('GET', missing));
      expect(res.status, 404);
      expect(res.body, 'none');
    });

    test('応答が得られない時は HttpFailure を投げる', () async {
      final h = create((responses: {}, failures: {'GET $down': 'refused'}));
      await expectLater(
        h.http.request(HttpRequest('GET', down)),
        throwsA(
          isA<HttpFailure>().having((e) => e.message, 'message', 'refused'),
        ),
      );
      expect(h.sent(), hasLength(1));
    });
  });
}

/// Storage の契約。create は 空の Storage を返す。
void storageContract(String name, Storage Function() create) {
  group('TC-OUT-CONTRACT Storage ($name)', () {
    test('無い key は null', () async {
      expect(await create().get('k'), isNull);
    });

    test('書いた値が読める。上書きは 後の値', () async {
      final s = create();
      await s.set('k', 'v1');
      expect(await s.get('k'), 'v1');
      await s.set('k', 'v2');
      expect(await s.get('k'), 'v2');
    });

    test('remove した key は null。他の key は残る。無い key の remove は 何もしない', () async {
      final s = create();
      await s.set('a', '1');
      await s.set('b', '2');
      await s.remove('a');
      await s.remove('zz');
      expect(await s.get('a'), isNull);
      expect(await s.get('b'), '2');
    });

    test('作るたびに 空から始まる', () async {
      await create().set('k', 'v');
      expect(await create().get('k'), isNull);
    });
  });
}

/// Config の契約。create は 表を受けて Config を返す。
void configContract(String name, Config Function(Map<String, String>) create) {
  group('TC-OUT-CONTRACT Config ($name)', () {
    test('表に在る名前は値、無い名前は null', () {
      final c = create({'API_URL': 'https://example.test', 'EMPTY': ''});
      expect(c.get('API_URL'), 'https://example.test');
      expect(c.get('EMPTY'), '');
      expect(c.get('NOT_DECLARED'), isNull);
    });

    test('渡した後に 元の表を変えても 値は変わらない', () {
      final table = {'A': '1'};
      final c = create(table);
      table['A'] = '2';
      table['B'] = '3';
      expect(c.get('A'), '1');
      expect(c.get('B'), isNull);
    });
  });
}
