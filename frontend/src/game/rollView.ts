import type { GameState } from '../protocol'

/**
 * 一投的可视化状态：投掷者离线预演出一整段关键帧，全桌（含投掷者）回放同一份。
 * 抽成纯函数，方便 store 保持精简、也方便单独测试。
 */
export interface RollView {
  trajectory: number[] | null
  trajectoryDice: number // 轨迹里每帧包含几颗骰子
  trajectoryRollId: number // 轨迹属于哪一次投掷
  trajectoryLive: boolean // 本端在投掷开始时就已在线（才从头播动画）
  stepsPerFrame: number
}

export const emptyRollView: RollView = {
  trajectory: null,
  trajectoryDice: 0,
  trajectoryRollId: 0,
  trajectoryLive: false,
  stepsPerFrame: 3,
}

/** 服务端请求投掷：先清掉上一轮，等投掷者把关键帧算完 */
export function rollViewOnRollRequested(rollId: number, diceCount: number): RollView {
  return {
    trajectory: null,
    trajectoryDice: diceCount,
    trajectoryRollId: rollId,
    trajectoryLive: true,
    stepsPerFrame: 3,
  }
}

/**
 * 收到状态广播时决定这一轮的可视化轨迹：
 * - 同一轮里必须沿用同一个数组引用，否则每次状态广播都会把投掷动画重播一遍；
 * - 只有本轮开局就在线的客户端才从头播，中途补看的人直接停在末帧。
 */
export function rollViewFromState(prev: RollView, state: GameState, rollingCount: number): RollView {
  const sameRoll = prev.trajectoryRollId === state.rollId && state.rollId !== 0
  const incoming = state.trajectory && state.trajectory.length > 0 ? state.trajectory : null
  const adopt = incoming !== null && (prev.trajectory === null || !sameRoll)

  if (!adopt) {
    return {
      trajectory: sameRoll ? prev.trajectory : null,
      trajectoryDice: sameRoll ? prev.trajectoryDice : rollingCount || 6,
      trajectoryRollId: state.rollId,
      trajectoryLive: sameRoll ? prev.trajectoryLive : false,
      stepsPerFrame: sameRoll ? prev.stepsPerFrame : 3,
    }
  }
  return {
    trajectory: incoming,
    trajectoryDice: state.trajectoryDice ?? (rollingCount || 6),
    trajectoryRollId: state.rollId,
    trajectoryLive: sameRoll,
    stepsPerFrame: state.stepsPerFrame ?? 3,
  }
}
