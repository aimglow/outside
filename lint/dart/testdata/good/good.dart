// 外は 口として受け取る。決めた時刻の DateTime は外ではない。
void good(dynamic clock, dynamic http, dynamic storage, dynamic config) {
  final now = clock.now();
  final fixed = DateTime.utc(2000);
  http.request(config.get('API'));
  storage.get('x');
  print([now, fixed]);
}
