// outside の Dart の lint (設計書 DSG-outside-001 の lint.dart)。
// 構文木を読み 外を直に呼ぶ code を名指す。部品の real の library は 対象に渡さない (取り込む project は real を持たない)。
import 'package:analyzer/dart/analysis/utilities.dart';
import 'package:analyzer/dart/ast/ast.dart';
import 'package:analyzer/dart/ast/visitor.dart';

/// 1 件の違反。
class Finding {
  Finding(this.line, this.what, this.port);
  final int line;
  final String what;
  final String port;
  @override
  String toString() => '$line: $what — 外を直に呼んでいる。outside の $port の口を通す (test では mock を渡す)';
}

const _staticCalls = <String, Map<String, String>>{
  'DateTime': {'now': 'Clock (now)'},
  'Process': {'run': 'Proc', 'runSync': 'Proc', 'start': 'Proc'},
  'Directory': {'systemTemp': 'Workdir', 'current': 'FS'},
  'Platform': {'environment': 'Config'},
};

const _constructors = <String, String>{
  'File': 'FS',
  'Directory': 'FS',
  'HttpClient': 'Http',
  'Timer': 'Clock (setTimeout)',
};

/// [source] を読み 違反を行の順で返す。読めない source は 例外。
List<Finding> check(String source) {
  final result = parseString(content: source, throwIfDiagnostics: true);
  final v = _Visitor(result.lineInfo.getLocation);
  result.unit.accept(v);
  v.found.sort((a, b) => a.line.compareTo(b.line));
  return v.found;
}

class _Visitor extends RecursiveAstVisitor<void> {
  _Visitor(this.loc);
  final dynamic Function(int) loc;
  final found = <Finding>[];

  int _line(AstNode n) => loc(n.offset).lineNumber as int;

  void _member(AstNode n, String? target, String name) {
    final port = _staticCalls[target]?[name];
    if (port != null) found.add(Finding(_line(n), '$target.$name', port));
  }

  @override
  void visitMethodInvocation(MethodInvocation node) {
    final t = node.target;
    if (t is SimpleIdentifier) {
      _member(node, t.name, node.methodName.name);
    } else if (t == null && _constructors.containsKey(node.methodName.name)) {
      // 型の解決をしない構文木では new の無い File(...) は 呼び出しに見える。
      found.add(Finding(_line(node), '${node.methodName.name}(...)', _constructors[node.methodName.name]!));
    }
    super.visitMethodInvocation(node);
  }

  @override
  void visitPrefixedIdentifier(PrefixedIdentifier node) {
    if (node.parent is! MethodInvocation) _member(node, node.prefix.name, node.identifier.name);
    super.visitPrefixedIdentifier(node);
  }

  @override
  void visitInstanceCreationExpression(InstanceCreationExpression node) {
    final name = node.constructorName.type.name.lexeme;
    final ctor = node.constructorName.name?.name;
    if (ctor != null && _staticCalls[name]?[ctor] != null) {
      found.add(Finding(_line(node), '$name.$ctor', _staticCalls[name]![ctor]!));
    } else if (_constructors.containsKey(name)) {
      found.add(Finding(_line(node), '$name(...)', _constructors[name]!));
    }
    super.visitInstanceCreationExpression(node);
  }
}
