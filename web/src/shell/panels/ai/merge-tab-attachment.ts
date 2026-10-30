/**
 * 再拖入一张页签时，先收下当前自动芯片。
 * 否则新页签会变成唯一工作区，原来的连接不会进这轮上下文。
 */
export function retainDisplacedTab<T extends { id: string; kind: string }>(
  list: T[],
  incoming: T,
  current: T | null | undefined,
): T[] {
  if (list.some((item) => item.id === incoming.id)) {
    return list
  }
  const next = [...list]
  if (
    incoming.kind === 'tab' &&
    current &&
    current.kind === 'tab' &&
    current.id !== incoming.id &&
    !next.some((item) => item.id === current.id)
  ) {
    next.push(current)
  }
  next.push(incoming)
  return next
}

/** 发送时并入当前激活页签（已有同 id 则不重复）。 */
export function mergeActiveTabAttachment<T extends { id: string }>(
  list: T[],
  active: T | null | undefined,
): T[] {
  if (!active) {
    return list
  }
  if (list.some((a) => a.id === active.id)) {
    return list
  }
  return [active, ...list]
}
