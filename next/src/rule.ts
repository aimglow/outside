// real と mock が共に使う FS の path の決めと 誤りの形。外は触らない。

/** FS・Proc の誤り。code で種類が判る (node の誤りの code と同じ名前を使う)。 */
export interface OutsideError extends Error {
  code: string;
}

/** code を持つ誤りを作る。 */
export function outsideError(code: string, message: string): OutsideError {
  const err = new Error(message) as OutsideError;
  err.code = code;
  return err;
}

/** 根の外を指す path の誤りの code。 */
export const OUTSIDE_ROOT = 'EOUTSIDEROOT';

/** 根の宣言が無い FS を使った時の誤りの code。 */
export const NO_ROOT = 'ENOROOT';

/** 宣言に無い program を起こそうとした時の誤りの code。 */
export const UNDECLARED_PROGRAM = 'EUNDECLARED';

/**
 * 根からの相対 path を整える (区切りは /)。根そのものは '.'。
 * 空・絶対 path (/ か \ か drive の文字で始まる)・根の外へ出る path は 誤り。
 */
export function cleanRel(p: string): string {
  if (p === '') throw outsideError(OUTSIDE_ROOT, 'outside FS: empty path');
  if (p.startsWith('/') || p.startsWith('\\') || /^[A-Za-z]:/.test(p)) {
    throw outsideError(OUTSIDE_ROOT, `outside FS: path outside the root: ${p}`);
  }
  const parts: string[] = [];
  for (const seg of p.split('/')) {
    if (seg === '' || seg === '.') continue;
    if (seg === '..') {
      if (parts.length === 0) {
        throw outsideError(OUTSIDE_ROOT, `outside FS: path outside the root: ${p}`);
      }
      parts.pop();
      continue;
    }
    parts.push(seg);
  }
  return parts.length === 0 ? '.' : parts.join('/');
}

/** 整えた相対 path の最後の名前 (根は '.')。 */
export function baseName(rel: string): string {
  const i = rel.lastIndexOf('/');
  return i < 0 ? rel : rel.slice(i + 1);
}

/** 整えた相対 path の親 (根の直下の名前と 根は '.')。 */
export function parentOf(rel: string): string {
  const i = rel.lastIndexOf('/');
  return i < 0 ? '.' : rel.slice(0, i);
}

/** 名前の順 (UTF-16 の code unit の順。機体の locale に依らない)。 */
export function byName(a: { name: string }, b: { name: string }): number {
  return a.name < b.name ? -1 : a.name > b.name ? 1 : 0;
}

/** stdin・stdout の値を byte 列にする (文字列は UTF-8)。渡した Uint8Array は写す。 */
export function toBytes(data: Uint8Array | string): Uint8Array {
  return typeof data === 'string' ? new TextEncoder().encode(data) : new Uint8Array(data);
}

/** abort で止めた時の誤り (name = 'AbortError'、code = 'ABORT_ERR')。 */
export function abortError(name: string): OutsideError {
  const err = outsideError('ABORT_ERR', `outside Proc: program ${name} was aborted`);
  err.name = 'AbortError';
  return err;
}

/** 宣言に無い program の誤り。 */
export function undeclaredProgram(name: string): OutsideError {
  return outsideError(UNDECLARED_PROGRAM, `outside Proc: undeclared program: ${name}`);
}

/** 根は消せない誤り。 */
export function rootRemoval(): OutsideError {
  return outsideError('EPERM', 'outside FS: remove .: the root cannot be removed');
}

/** FS の op が path で失敗した誤り (node の誤りの code と同じ code)。 */
export function fsError(code: string, op: string, rel: string): OutsideError {
  return outsideError(code, `outside FS: ${op} ${rel}: ${code}`);
}
