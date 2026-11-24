import 'dart:async';
import 'dart:io';

void bad() {
  final a = DateTime.now();
  final b = Platform.environment['HOME'];
  final c = File('x.txt');
  final d = Directory.systemTemp;
  Process.run('git', ['status']);
  final e = HttpClient();
  Timer(const Duration(seconds: 1), () {});
  final f = new File('y.txt');
  print([a, b, c, d, e, f]);
}
