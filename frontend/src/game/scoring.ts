// Farkle 计分规则的客户端镜像（仅用于 UI 预览，权威结果由服务端决定）
export interface ScoreResult {
  baseScore: number
  isFarkle: boolean
}

export function scoreDice(dice: number[]): ScoreResult {
  if (dice.length === 0) return { baseScore: 0, isFarkle: true }
  const counts = new Array(7).fill(0)
  for (const d of dice) {
    if (d < 1 || d > 6) return { baseScore: 0, isFarkle: true }
    counts[d]++
  }

  const nonZero = counts.slice(1).filter((c: number) => c > 0).length

  // 1-6 顺子
  if (nonZero === 6) return { baseScore: 1500, isFarkle: false }

  // 三对
  let pairs = 0
  for (let i = 1; i <= 6; i++) if (counts[i] === 2) pairs++
  if (pairs === 3) return { baseScore: 1500, isFarkle: false }

  // 1-5 顺子（5 颗）
  if (
    dice.length === 5 &&
    nonZero === 5 &&
    counts[2] === 1 && counts[3] === 1 && counts[4] === 1 && counts[5] === 1 && counts[6] === 1
  ) {
    return { baseScore: 1500, isFarkle: false }
  }

  // 两三条
  let triples = 0
  for (let i = 1; i <= 6; i++) if (counts[i] === 3) triples++
  if (triples === 2) return { baseScore: 2500, isFarkle: false }

  let score = 0
  const remaining = counts.slice()
  for (let face = 1; face <= 6; face++) {
    const n = counts[face]
    if (n < 3) continue
    let base = face * 100
    if (face === 1) base = 1000
    let mult = 1
    for (let extra = n - 3; extra > 0; extra--) mult *= 2
    score += base * mult
    remaining[face] = 0
  }
  score += remaining[1] * 100 + remaining[5] * 50

  if (score === 0) return { baseScore: 0, isFarkle: true }
  return { baseScore: score, isFarkle: false }
}

/** 从散骰池中移除所选骰子（多重集减法） */
export function removeSelected(pool: number[], selected: number[]): number[] | null {
  const rest = pool.slice()
  for (const s of selected) {
    const idx = rest.indexOf(s)
    if (idx === -1) return null
    rest.splice(idx, 1)
  }
  return rest
}
