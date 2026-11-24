// TC-OUT-LINT-DART: 外を直に呼ぶ code を落とし 口を通す code を通す。
import 'dart:io';

import 'package:outside_lint/outside_lint.dart';
import 'package:test/test.dart';

String fixture(String name) => File('testdata/$name').readAsStringSync();

void main() {
  test('外を直に呼ぶ code を落とす (禁じた 8 の呼び出しが全部 名指される)', () {
    final got = check(fixture('bad/bad.dart')).map((f) => '${f.line}:${f.what}').toList();
    expect(got, [
      '5:DateTime.now',
      '6:Platform.environment',
      '7:File(...)',
      '8:Directory.systemTemp',
      '9:Process.run',
      '10:HttpClient(...)',
      '11:Timer(...)',
      '12:File(...)',
    ]);
  });

  test('口を通す code と 決めた時刻の DateTime は通す', () {
    expect(check(fixture('good/good.dart')), isEmpty);
  });

  test('読めない source は 例外 (判定不能)', () {
    expect(() => check('void x( {'), throwsA(anything));
  });
}
