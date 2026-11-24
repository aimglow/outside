// TC-OUT-RUNTIME-DEPS: 部品 flutter の依存の宣言と lib の import が dart:* の外を持たない。
// pubspec.yaml と lib の source を そのまま読んで確かめる (この部品の中の file だけを読む)。
import 'dart:convert';
import 'dart:io';

import 'package:test/test.dart';

/// pubspec の top level の dependencies の項目の名前。
List<String> _dependencies(String pubspec) {
  final names = <String>[];
  var inside = false;
  int? childIndent;
  for (final line in const LineSplitter().convert(pubspec)) {
    if (line.trim().isEmpty || line.trimLeft().startsWith('#')) continue;
    final topLevel = !line.startsWith(' ') && !line.startsWith('\t');
    if (topLevel) {
      final m = RegExp(r'^dependencies\s*:\s*(.*)$').firstMatch(line);
      inside = m != null;
      childIndent = null;
      if (m != null) {
        final rest = m.group(1)!.trim();
        // 1 行の形 (dependencies: {a: ^1.0.0}) も 名前を拾う。
        if (rest.isNotEmpty && rest != '{}') {
          names.addAll(
            RegExp(
              r'([A-Za-z_][A-Za-z0-9_]*)\s*:',
            ).allMatches(rest).map((x) => x.group(1)!),
          );
        }
      }
      continue;
    }
    if (inside) {
      // 子の段 (最初の項目の字下げ) の名前だけを拾う。孫 (sdk: flutter 等) は拾わない。
      final m = RegExp(r'^(\s+)([A-Za-z_][A-Za-z0-9_]*)\s*:').firstMatch(line);
      if (m == null) continue;
      childIndent ??= m.group(1)!.length;
      if (m.group(1)!.length == childIndent) names.add(m.group(2)!);
    }
  }
  return names;
}

/// source の import・export・part の指定子。
List<String> _directives(String source) => [
  for (final m in RegExp(
    r'''^\s*(?:import|export|part)\s+['"]([^'"]+)['"]''',
    multiLine: true,
  ).allMatches(source))
    m.group(1)!,
];

/// lib が使ってよい指定子 = dart:*・package:outside/・相対。
bool _allowed(String spec) =>
    spec.startsWith('dart:') ||
    spec.startsWith('package:outside/') ||
    !spec.contains(':');

void main() {
  group('TC-OUT-RUNTIME-DEPS 部品 flutter の依存', () {
    test('取り出しの陽性対照: 依存と禁じた import を 書き方に依らず拾う', () {
      const pubspec = '''
name: x
dependencies:
  http: ^1.2.0
  flutter:
    sdk: flutter
dev_dependencies:
  test: ^1.25.0
''';
      expect(_dependencies(pubspec), ['http', 'flutter']);
      expect(_dependencies('name: x\ndependencies: {}\n'), isEmpty);
      expect(_dependencies('dependencies: {http: ^1.0.0}\n'), ['http']);
      const source = '''
import 'dart:io';
import "package:http/http.dart" as http;
export 'package:flutter/widgets.dart';
import 'package:outside/outside.dart';
part 'a.dart';
''';
      final specs = _directives(source);
      expect(specs, [
        'dart:io',
        'package:http/http.dart',
        'package:flutter/widgets.dart',
        'package:outside/outside.dart',
        'a.dart',
      ]);
      expect(specs.where((s) => !_allowed(s)), [
        'package:http/http.dart',
        'package:flutter/widgets.dart',
      ]);
    });

    test('pubspec.yaml の dependencies は空', () {
      expect(_dependencies(File('pubspec.yaml').readAsStringSync()), isEmpty);
    });

    test('lib の import は dart:* と package:outside と 相対だけ', () {
      final files =
          Directory('lib')
              .listSync(recursive: true)
              .whereType<File>()
              .where((f) => f.path.endsWith('.dart'))
              .toList()
            ..sort((a, b) => a.path.compareTo(b.path));
      expect(files, isNotEmpty);
      final outside = [
        for (final f in files)
          for (final spec in _directives(f.readAsStringSync()))
            if (!_allowed(spec)) '${f.path}: $spec',
      ];
      expect(outside, isEmpty);
    });
  });
}
