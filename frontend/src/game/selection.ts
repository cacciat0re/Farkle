/**
 * 当前物理骰子数组中，哪些索引已经被锁定并从剩余骰池移除。
 * state.diceValues 是服务端锁定后的剩余骰池，不能用整个回合累积的 lockedDice
 * 按值匹配，否则新一轮出现同点数时会把有效骰子误隐藏。
 */
export function hiddenIndices(
  settledValues: (number | null)[],
  remainingDice: number[],
): Set<number> {
  const remaining = new Map<number, number>()
  for (const value of remainingDice) {
    remaining.set(value, (remaining.get(value) ?? 0) + 1)
  }

  const hidden = new Set<number>()
  settledValues.forEach((value, index) => {
    // 观战端可能先收到服务端 scoring 状态，本地物理还没停稳。
    // 未停稳的骰子不能提前隐藏，否则刚体会被禁用，永远无法完成结算。
    if (value == null) return
    const count = remaining.get(value) ?? 0
    if (count > 0) {
      remaining.set(value, count - 1)
    } else {
      hidden.add(index)
    }
  })
  return hidden
}
