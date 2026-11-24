/// 口ごとの mock。test が渡した値だけで振る舞いが決まる。
library;

import 'dart:convert' show utf8;
import 'dart:typed_data' show Uint8List;

import 'outside.dart';

/// 決めた時刻から始まり [advance] でだけ進む Clock。
final class MockClock implements Clock {
  MockClock(DateTime start) : _now = start.toUtc();

  DateTime _now;
  int _nextId = 0;
  final Map<int, ({DateTime due, void Function() callback})> _pending = {};

  /// now が呼ばれた回数。
  int nowCalls = 0;

  /// setTimeout に渡された delay (呼ばれた順)。
  final List<Duration> scheduled = [];

  /// clearTimeout に渡された札 (呼ばれた順)。
  final List<TimeoutHandle> cleared = [];

  /// まだ発火していない timer の数。
  int get pendingTimeouts => _pending.length;

  @override
  DateTime now() {
    nowCalls++;
    return _now;
  }

  @override
  TimeoutHandle setTimeout(Duration delay, void Function() callback) {
    scheduled.add(delay);
    final id = _nextId++;
    final wait = delay.isNegative ? Duration.zero : delay;
    _pending[id] = (due: _now.add(wait), callback: callback);
    return TimeoutHandle(id);
  }

  @override
  void clearTimeout(TimeoutHandle handle) {
    cleared.add(handle);
    _pending.remove(handle.id);
  }

  /// 時刻を d だけ進め、期限が来た timer を 期限の早い順 (同じなら掛けた順) に呼ぶ。
  void advance(Duration d) {
    if (d.isNegative) throw ArgumentError.value(d, 'd', '負の長さは進められない');
    final end = _now.add(d);
    while (true) {
      int? nextId;
      DateTime? nextDue;
      for (final e in _pending.entries) {
        final due = e.value.due;
        if (due.isAfter(end)) continue;
        if (nextDue == null || due.isBefore(nextDue)) {
          nextId = e.key;
          nextDue = due;
        }
      }
      if (nextId == null) break;
      final callback = _pending.remove(nextId)!.callback;
      if (nextDue!.isAfter(_now)) _now = nextDue;
      callback();
    }
    if (end.isAfter(_now)) _now = end;
  }
}

/// 決めた応答を返し 送られた内容を記録する Http。
final class MockHttp implements Http {
  final Map<String, HttpResponse> _responses = {};
  final Map<String, String> _failures = {};

  /// 送られた内容 (呼ばれた順)。
  final List<HttpRequest> requests = [];

  static String _key(String method, Uri url) => '${method.toUpperCase()} $url';

  /// method と url の組に返す応答を決める。
  void respond(String method, Uri url, HttpResponse response) {
    _failures.remove(_key(method, url));
    _responses[_key(method, url)] = response;
  }

  /// method と url の組を 応答が得られない誤りにする。
  void fail(String method, Uri url, String message) {
    _responses.remove(_key(method, url));
    _failures[_key(method, url)] = message;
  }

  @override
  Future<HttpResponse> request(HttpRequest request) async {
    requests.add(request);
    final key = _key(request.method, request.url);
    final failure = _failures[key];
    if (failure != null) throw HttpFailure(failure);
    final response = _responses[key];
    if (response == null) throw StateError('MockHttp に応答が決められていない: $key');
    return response;
  }
}

/// memory の上の表で持つ Storage。空から始まる。
final class MockStorage implements Storage {
  final Map<String, String> _values = {};

  /// 呼ばれた内容 (呼ばれた順)。op は get・set・remove。
  final List<({String op, String key, String? value})> calls = [];

  /// 今の中身の写し。
  Map<String, String> get values => Map.unmodifiable(_values);

  @override
  Future<String?> get(String key) async {
    calls.add((op: 'get', key: key, value: null));
    return _values[key];
  }

  @override
  Future<void> set(String key, String value) async {
    calls.add((op: 'set', key: key, value: value));
    _values[key] = value;
  }

  @override
  Future<void> remove(String key) async {
    calls.add((op: 'remove', key: key, value: null));
    _values.remove(key);
  }
}

/// 渡した表だけを返す Config。
final class MockConfig implements Config {
  MockConfig([Map<String, String> table = const {}]) : _table = Map.of(table);

  final Map<String, String> _table;

  /// get に渡された名前 (呼ばれた順)。
  final List<String> lookups = [];

  @override
  String? get(String name) {
    lookups.add(name);
    return _table[name];
  }
}

/// 'id-0001'・'id-0002' … と 呼んだ順に決まった ID を返す IdGen。作るたびに 1 から始まる。
final class MockIdGen implements IdGen {
  int _calls = 0;

  /// newId が呼ばれた回数。
  int get calls => _calls;

  @override
  String newId() {
    _calls++;
    return 'id-${_calls.toString().padLeft(4, '0')}';
  }
}

/// 種で決まる byte 列を返す Random。同じ種・同じ呼び順なら 同じ byte。
///
/// 列 = 種の byte 列の FNV-1a 64bit を初期値とする splitmix64 の出力を 1 つずつ 8 byte (little endian) に開いて並べた物
/// (next・angular の createMockRandom と同じ列)。1 本の byte の流れとして持つので 分けて読んでも 続けて読んでも 同じ列。
/// 64bit の整数の演算 (2 の 64 乗で回る) を使うため native (iOS・Android・機体の dart) で動かす。
final class MockRandom implements Random {
  /// seed の byte 列 (各要素の下 8bit) を種にする。
  MockRandom(List<int> seed) : _state = _fnv1a64(seed);

  /// 文字列の種。UTF-8 の byte 列を種にする。
  MockRandom.text(String seed) : this(utf8.encode(seed));

  int _state;
  final Uint8List _block = Uint8List(8);
  int _pos = 8;
  Object? _err;

  /// read に渡された buf の長さ (読んだ順)。
  final List<int> calls = [];

  static int _fnv1a64(List<int> bytes) {
    var h = 0xcbf29ce484222325;
    for (final b in bytes) {
      h = (h ^ (b & 0xff)) * 0x100000001b3;
    }
    return h;
  }

  void _nextBlock() {
    _state += 0x9e3779b97f4a7c15;
    var z = _state;
    z = (z ^ (z >>> 30)) * 0xbf58476d1ce4e5b9;
    z = (z ^ (z >>> 27)) * 0x94d049bb133111eb;
    z ^= z >>> 31;
    for (var i = 0; i < 8; i++) {
      _block[i] = (z >>> (8 * i)) & 0xff;
    }
    _pos = 0;
  }

  /// 以後の read が err を投げるようにする。null を渡すと 列を返す形に戻す。
  MockRandom withErr(Object? err) {
    _err = err;
    return this;
  }

  @override
  void read(Uint8List buf) {
    calls.add(buf.length);
    final err = _err;
    if (err != null) throw err;
    for (var i = 0; i < buf.length; i++) {
      if (_pos == 8) _nextBlock();
      buf[i] = _block[_pos++];
    }
  }
}
