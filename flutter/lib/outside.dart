/// 外 (時計・通信・保存・設定) を触る口と 本物の実装。
///
/// 設計書の ports.frontend_and_mobile との対応 (名前は設計書と同じ):
/// - Clock: now / setTimeout / clearTimeout
/// - Http: request
/// - Storage: get / set / remove (端末の保存は非同期ゆえ Future を返す)
/// - Config: get
///
/// 実の時計・timer・環境変数・通信を呼ぶのは この library の Real* だけ。
library;

import 'dart:async';
import 'dart:convert';
import 'dart:io' show Platform;

import 'package:http/http.dart' as http;

/// setTimeout が返す札。clearTimeout に渡す。
final class TimeoutHandle {
  const TimeoutHandle(this.id);

  final int id;
}

/// 今の時刻を返す。timer を掛ける。
abstract interface class Clock {
  /// 今の時刻を UTC で返す。
  DateTime now();

  /// delay の後に callback を 1 回呼ぶ。負の delay は 0 として扱う。
  TimeoutHandle setTimeout(Duration delay, void Function() callback);

  /// 掛けた timer を取り消す。済んだ札・知らない札は 何もしない。
  void clearTimeout(TimeoutHandle handle);
}

/// 送る通信の内容。
final class HttpRequest {
  /// body と bodyBytes は どちらか片方を渡す。body は UTF-8 で送る。
  HttpRequest(
    String method,
    this.url, {
    Map<String, String> headers = const {},
    String? body,
    List<int>? bodyBytes,
  }) : method = method.toUpperCase(),
       headers = Map.unmodifiable(headers),
       bodyBytes = List.unmodifiable(bodyBytes ?? utf8.encode(body ?? '')) {
    if (body != null && bodyBytes != null) {
      throw ArgumentError('body と bodyBytes は 片方だけ渡す');
    }
  }

  /// 大文字の method (GET・POST 等)。
  final String method;
  final Uri url;
  final Map<String, String> headers;
  final List<int> bodyBytes;

  /// body を UTF-8 の文字列として読む。
  String get body => utf8.decode(bodyBytes);
}

/// 返ってきた応答。
final class HttpResponse {
  /// header の名前は 小文字に揃える。
  HttpResponse(
    this.status, {
    Map<String, String> headers = const {},
    String? body,
    List<int>? bodyBytes,
  }) : headers = Map.unmodifiable({
         for (final e in headers.entries) e.key.toLowerCase(): e.value,
       }),
       bodyBytes = List.unmodifiable(bodyBytes ?? utf8.encode(body ?? '')) {
    if (body != null && bodyBytes != null) {
      throw ArgumentError('body と bodyBytes は 片方だけ渡す');
    }
  }

  final int status;
  final Map<String, String> headers;
  final List<int> bodyBytes;

  /// body を UTF-8 の文字列として読む。
  String get body => utf8.decode(bodyBytes);
}

/// 応答が得られなかった誤り (接続できない等)。status の付いた応答は 誤りにしない。
final class HttpFailure implements Exception {
  const HttpFailure(this.message);

  final String message;

  @override
  String toString() => 'HttpFailure: $message';
}

/// 通信を送り 応答を返す。
abstract interface class Http {
  /// 応答を返す。応答が得られない時は [HttpFailure] を投げる。
  Future<HttpResponse> request(HttpRequest request);
}

/// 端末の保存を読む・書く。
abstract interface class Storage {
  /// key の値を返す。無ければ null。
  Future<String?> get(String key);

  /// key に value を書く。在れば上書きする。
  Future<void> set(String key, String value);

  /// key を消す。無ければ 何もしない。
  Future<void> remove(String key);
}

/// 設定の値を返す。
abstract interface class Config {
  /// name の値を返す。無ければ null。
  String? get(String name);
}

/// 1 つの処理に渡す口の組。
final class OutsideSet {
  const OutsideSet({
    required this.clock,
    required this.http,
    required this.storage,
    required this.config,
  });

  /// 本物の口の組を作る。保存の実体と build 時の設定の表は app が渡す。
  factory OutsideSet.real({
    required StorageBackend storageBackend,
    Map<String, String> buildTimeConfig = const {},
  }) => OutsideSet(
    clock: RealClock(),
    http: RealHttp(),
    storage: RealStorage(storageBackend),
    config: RealConfig(buildTime: buildTimeConfig),
  );

  final Clock clock;
  final Http http;
  final Storage storage;
  final Config config;
}

/// 実の時計と実の timer を使う Clock。
final class RealClock implements Clock {
  final Map<int, Timer> _timers = {};
  int _nextId = 0;

  @override
  DateTime now() => DateTime.now().toUtc();

  @override
  TimeoutHandle setTimeout(Duration delay, void Function() callback) {
    final id = _nextId++;
    _timers[id] = Timer(delay, () {
      _timers.remove(id);
      callback();
    });
    return TimeoutHandle(id);
  }

  @override
  void clearTimeout(TimeoutHandle handle) {
    _timers.remove(handle.id)?.cancel();
  }
}

/// package:http の client で送る Http。client を渡さなければ 既定の client を使う。
final class RealHttp implements Http {
  RealHttp({http.Client? client}) : _client = client ?? http.Client();

  final http.Client _client;

  @override
  Future<HttpResponse> request(HttpRequest request) async {
    final req = http.Request(request.method, request.url)
      ..headers.addAll(request.headers)
      ..bodyBytes = request.bodyBytes;
    try {
      final res = await http.Response.fromStream(await _client.send(req));
      return HttpResponse(
        res.statusCode,
        headers: res.headers,
        bodyBytes: res.bodyBytes,
      );
    } on http.ClientException catch (e) {
      throw HttpFailure(e.message);
    }
  }
}

/// 保存の実体。Flutter の app が 端末の保存 (shared_preferences 等) で実装して渡す。
abstract interface class StorageBackend {
  Future<String?> get(String key);
  Future<void> set(String key, String value);
  Future<void> remove(String key);
}

/// 渡された実体へ読み書きする Storage。
final class RealStorage implements Storage {
  RealStorage(this._backend);

  final StorageBackend _backend;

  @override
  Future<String?> get(String key) => _backend.get(key);

  @override
  Future<void> set(String key, String value) => _backend.set(key, value);

  @override
  Future<void> remove(String key) => _backend.remove(key);
}

/// build 時の表と 実行時の環境変数から読む Config。同じ名前は 実行時の値が勝つ。
///
/// build 時の値は app が const の表で渡す (String.fromEnvironment は const でしか読めない)。
/// runtime を渡さなければ 機体の環境変数を読む。
final class RealConfig implements Config {
  RealConfig({
    Map<String, String> buildTime = const {},
    Map<String, String>? runtime,
  }) : _buildTime = Map.of(buildTime),
       _runtime = runtime == null ? null : Map.of(runtime);

  final Map<String, String> _buildTime;
  final Map<String, String>? _runtime;

  @override
  String? get(String name) =>
      (_runtime ?? Platform.environment)[name] ?? _buildTime[name];
}
