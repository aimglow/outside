// 使い方: dart run outside_lint:check <dir か file> ...
// 違反が在れば名指して exit 1、読めない file は exit 3 (判定不能)、無ければ 0。
import 'dart:io';

import 'package:outside_lint/outside_lint.dart';

void main(List<String> args) {
  var bad = 0;
  var unreadable = 0;
  final files = <File>[];
  for (final a in args) {
    final d = Directory(a);
    if (d.existsSync()) {
      files.addAll(d.listSync(recursive: true).whereType<File>().where((f) => f.path.endsWith('.dart')));
    } else {
      files.add(File(a));
    }
  }
  files.sort((a, b) => a.path.compareTo(b.path));
  for (final f in files) {
    try {
      for (final x in check(f.readAsStringSync())) {
        stdout.writeln('${f.path}:$x');
        bad++;
      }
    } catch (e) {
      stderr.writeln('${f.path}: 読めない (判定不能): $e');
      unreadable++;
    }
  }
  if (unreadable > 0) exit(3);
  exit(bad > 0 ? 1 : 0);
}
