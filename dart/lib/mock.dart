/// 口ごとの mock。test が渡した値だけで振る舞いが決まる。
library;

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
