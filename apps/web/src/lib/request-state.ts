export interface LatestRequestGate {
  begin: () => number;
  isCurrent: (requestId: number) => boolean;
}

export function createLatestRequestGate(): LatestRequestGate {
  let latestRequestId = 0;

  return {
    begin() {
      latestRequestId += 1;
      return latestRequestId;
    },
    isCurrent(requestId) {
      return requestId === latestRequestId;
    },
  };
}

export function mergeUniqueById<T extends { id: string }>(
  current: readonly T[],
  incoming: readonly T[],
): T[] {
  const merged = [...current];
  const indexes = new Map(merged.map((item, index) => [item.id, index]));

  for (const item of incoming) {
    const existingIndex = indexes.get(item.id);
    if (existingIndex === undefined) {
      indexes.set(item.id, merged.length);
      merged.push(item);
    } else {
      merged[existingIndex] = item;
    }
  }

  return merged;
}

export function mergeNewestPageById<T extends { id: string }>(
  current: T[],
  newestPage: T[],
): T[] {
  const newestIds = new Set(newestPage.map((item) => item.id));
  return [...newestPage, ...current.filter((item) => !newestIds.has(item.id))];
}
