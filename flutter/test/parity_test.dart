// 公開している口と method の名前の組を確かめる。
import 'dart:mirrors';

import 'package:outside/outside.dart';
import 'package:test/test.dart';

/// 型が宣言している method の名前。
Set<String> _methods(Type type) => {
  for (final d in reflectClass(type).declarations.values)
    if (d is MethodMirror && !d.isConstructor && !d.isPrivate)
      MirrorSystem.getName(d.simpleName),
};

void main() {
  test('TC-OUT-PARITY 口と method の名前は 設計書の ports.frontend_and_mobile の組', () {
    expect(
      {
        'Clock': _methods(Clock),
        'Http': _methods(Http),
        'Storage': _methods(Storage),
        'Config': _methods(Config),
        'IdGen': _methods(IdGen),
      },
      {
        'Clock': {'now', 'setTimeout', 'clearTimeout'},
        'Http': {'request'},
        'Storage': {'get', 'set', 'remove'},
        'Config': {'get'},
        'IdGen': {'newId'},
      },
    );
  });
}
