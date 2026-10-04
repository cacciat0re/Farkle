// Farkle 计分规则的客户端镜像（须与服务端 game.go 保持一致）
export interface ScoreResult {
  baseScore: number
  isFarkle: boolean
}

/** 同服务端 scoreDice：单组骰子的基础分（含特殊组合），不保证每颗都参与得分 */
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

function removeFirstN(dice: number[], face: number, n: number): number[] {
  const rest: number[] = []
  let removed = 0
  for (const d of dice) {
    if (d === face && removed < n) {
      removed++
      continue
    }
    rest.push(d)
  }
  return rest
}

/** 特殊组合分数（dice 须恰好组成该组合） */
function specialScore(dice: number[]): number | null {
  const n = dice.length
  const counts = new Array(7).fill(0)
  for (const d of dice) counts[d]++
  const nonZero = counts.slice(1).filter((c: number) => c > 0).length

  if (n === 6 && nonZero === 6) return 1500
  if (n === 6) {
    let pairs = 0
    let triples = 0
    for (let i = 1; i <= 6; i++) {
      if (counts[i] === 2) pairs++
      if (counts[i] === 3) triples++
    }
    if (pairs === 3) return 1500
    if (triples === 2) return 2500
  }
  if (n === 5 && nonZero === 5) {
    if (counts[1] === 1 && counts[2] === 1 && counts[3] === 1 && counts[4] === 1 && counts[5] === 1) return 1500
    if (counts[2] === 1 && counts[3] === 1 && counts[4] === 1 && counts[5] === 1 && counts[6] === 1) return 1500
  }
  return null
}

/**
 * 完整分解计分：每颗骰子都必须参与得分，否则 ok=false。
 * 返回 [总分, 是否合法] —— 与服务端 scoreFull 一致
 */
export function scoreFull(dice: number[]): [number, boolean] {
  if (dice.length === 0) return [0, true]
  const sp = specialScore(dice)
  if (sp !== null) return [sp, true]

  const counts = new Array(7).fill(0)
  for (const d of dice) counts[d]++

  let best = 0
  let found = false
  const tryDecompose = (rest: number[], gained: number) => {
    const [r, ok] = scoreFull(rest)
    if (ok && (!found || r + gained > best)) {
      best = r + gained
      found = true
    }
  }

  if (counts[1] > 0) tryDecompose(removeFirstN(dice, 1, 1), 100)
  if (counts[5] > 0) tryDecompose(removeFirstN(dice, 5, 1), 50)
  for (let face = 1; face <= 6; face++) {
    if (counts[face] < 3) continue
    for (let n = 3; n <= counts[face]; n++) {
      let base = face * 100
      if (face === 1) base = 1000
      let mult = 1
      for (let e = n - 3; e > 0; e--) mult *= 2
      tryDecompose(removeFirstN(dice, face, n), base * mult)
    }
  }
  return [best, found]
}

/** 是否合法锁定（每颗都参与得分） */
export function isValidKeep(dice: number[]): boolean {
  if (dice.length === 0) return false
  return scoreFull(dice)[1]
}

/** 整组最佳得分；0 = Farkle */
export function bestScore(dice: number[]): number {
  let best = 0
  const n = dice.length
  for (let mask = 1; mask < 1 << n; mask++) {
    const sub: number[] = []
    for (let i = 0; i < n; i++) if (mask & (1 << i)) sub.push(dice[i])
    const [s, ok] = scoreFull(sub)
    if (ok && s > best) best = s
  }
  return best
}

/**
 * 在当前散骰池中，给定已选索引，计算"还能选哪些骰子"：
 * 仅当存在某个合法锁定 K 满足 selected ⊆ K 且 i ∈ K 时，骰子 i 才可选。
 * 这样用户永远无法选出夹带废骰的组合（如三条5加一颗2）。
 */
export function selectableIndices(pool: (number | null)[], selected: number[]): Set<number> {
  const n = pool.length
  const selSet = new Set(selected)
  // 枚举所有合法锁定（索引子集）
  const keeps: Set<number>[] = []
  for (let mask = 1; mask < 1 << n; mask++) {
    const idxs: number[] = []
    const vals: number[] = []
    let available = true
    for (let i = 0; i < n; i++) {
      if (mask & (1 << i)) {
        if (pool[i] == null) {
          available = false
          break
        }
        idxs.push(i)
        vals.push(pool[i]!)
      }
    }
    if (!available) continue
    if (!isValidKeep(vals)) continue
    const set = new Set(idxs)
    // 必须包含当前全部选择
    let contains = true
    for (const s of selSet) {
      if (!set.has(s)) {
        contains = false
        break
      }
    }
    if (contains) keeps.push(set)
  }
  const result = new Set<number>()
  for (const k of keeps) {
    for (const i of k) if (!selSet.has(i)) result.add(i)
  }
  return result
}
