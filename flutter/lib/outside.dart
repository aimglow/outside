/// 外 (時計・通信・保存・設定・ID) を触る口と 本物の実装。
///
/// 設計書の ports.frontend_and_mobile との対応 (名前は設計書と同じ):
/// - Clock: now / setTimeout / clearTimeout
/// - Http: request
/// - Storage: get / set / remove (端末の保存は非同期ゆえ Future を返す)
/// - Config: get
/// - IdGen: newId
///
/// 実の時計・timer・環境変数・通信・乱数を呼ぶのは この library の Real* だけ。
/// 依存は dart:* だけ (package:flutter も plugin も使わない)。
library;

import 'dart:async';
import 'dart:convert';
import 'dart:io' show HttpClient, IOException, Platform;
import 'dart:math' show Random;

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

/// 新しい ID を返す。
abstract interface class IdGen {
  /// 呼ぶたびに 違う ID を返す。空の文字列は返さない。
  String newId();
}

/// 1 つの処理に渡す口の組。
final class OutsideSet {
  const OutsideSet({
    required this.clock,
    required this.http,
    required this.storage,
    required this.config,
    required this.idGen,
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
    idGen: RealIdGen(),
  );

  final Clock clock;
  final Http http;
  final Storage storage;
  final Config config;
  final IdGen idGen;
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

/// dart:io の HttpClient で送る Http。
///
/// client を渡さなければ 最初の request の時に 既定の HttpClient を作る (作るだけでは 何も触らない)。
/// 同じ名前の header が複数届いた時は ', ' で繋いで 1 つの値にする。
final class RealHttp implements Http {
  RealHttp({HttpClient? client}) : _client = client;

  HttpClient? _client;

  @override
  Future<HttpResponse> request(HttpRequest request) async {
    final client = _client ??= HttpClient();
    try {
      final req = await client.openUrl(request.method, request.url);
      request.headers.forEach(req.headers.set);
      req.contentLength = request.bodyBytes.length;
      req.add(request.bodyBytes);
      final res = await req.close();
      final headers = <String, String>{};
      res.headers.forEach((name, values) => headers[name] = values.join(', '));
      final body = await res.fold<List<int>>(
        <int>[],
        (bytes, chunk) => bytes..addAll(chunk),
      );
      return HttpResponse(res.statusCode, headers: headers, bodyBytes: body);
    } on IOException catch (e) {
      // 接続できない・応答の途中で切れた・TLS の誤り。status の付いた応答は ここに来ない。
      throw HttpFailure(e.toString());
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

/// 安全な乱数 (Random.secure) で UUID v4 の形の ID を返す IdGen。
final class RealIdGen implements IdGen {
  RealIdGen() : _random = Random.secure();

  final Random _random;

  @override
  String newId() {
    final bytes = List<int>.generate(16, (_) => _random.nextInt(256));
    bytes[6] = (bytes[6] & 0x0f) | 0x40; // 版 4
    bytes[8] = (bytes[8] & 0x3f) | 0x80; // 変種 RFC 4122
    final hex = [
      for (final b in bytes) b.toRadixString(16).padLeft(2, '0'),
    ].join();
    return '${hex.substring(0, 8)}-${hex.substring(8, 12)}-${hex.substring(12, 16)}-'
        '${hex.substring(16, 20)}-${hex.substring(20)}';
  }
}
