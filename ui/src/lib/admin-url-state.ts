export function readListParam<T extends string>(
  params: URLSearchParams,
  key: string,
  allowed?: readonly T[],
): T[] {
  const raw = params.get(key);
  if (!raw) return [];
  const allowedSet = allowed ? new Set<string>(allowed) : null;
  const seen = new Set<string>();
  const values: T[] = [];
  for (const part of raw.split(",")) {
    const value = part.trim();
    if (!value || seen.has(value)) continue;
    if (allowedSet && !allowedSet.has(value)) continue;
    seen.add(value);
    values.push(value as T);
  }
  return values;
}

export function writeListParam(
  params: URLSearchParams,
  key: string,
  values: readonly string[],
) {
  const clean = values.map((v) => v.trim()).filter(Boolean);
  if (clean.length === 0) {
    params.delete(key);
    return;
  }
  params.set(key, clean.join(","));
}

export function writeTextParam(
  params: URLSearchParams,
  key: string,
  value: string | null | undefined,
) {
  const clean = value?.trim() ?? "";
  if (!clean) {
    params.delete(key);
    return;
  }
  params.set(key, clean);
}
