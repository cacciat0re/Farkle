export type Phase = 'waiting' | 'rolling' | 'scoring' | 'turnEnd' | 'gameOver'

export interface Player {
  id: string
  name: string
  score: number
  hasBanked: boolean
}

export interface ChatMessage {
  kind: 'chat' | 'system'
  playerId?: string
  name?: string
  text: string
  timestamp: number
}

/** 一颗骰子停稳后的姿态：位置 + 四元数 */
export interface DiePose {
  p: [number, number, number]
  q: [number, number, number, number]
}

export interface GameState {
  roomId: string
  phase: Phase
  players: Player[]
  currentTurn: number
  diceValues: number[]
  lockedDice: number[]
  turnScore: number
  rollCount: number
  winner?: string
  farkled: boolean
  lastBanked: number
  rollId: number // 每次投掷递增，前端据此触发本地动画
  animatingTill: number
  rematchReady: string[]
  readyPlayers: string[]
  keptThisRoll: boolean // 本次投掷后是否已锁定至少一颗骰子
  dicePoses?: DiePose[] // 本轮权威姿态（投掷者上报，全桌对齐用）
  trajectory?: number[] // 本轮关键帧轨迹（扁平：每帧 N×(x,y,z,qx,qy,qz,qw)）
  trajectoryDice?: number // 轨迹里每帧包含的骰子数（锁骰后可能少于 6）
  stepsPerFrame?: number // 每个关键帧间隔的物理步数
}

export type ClientMessage =
  | { type: 'JOIN_ROOM'; playerId: string; name: string; roomId: string }
  | { type: 'PLAYER_READY'; playerId: string; roomId: string }
  | { type: 'ROLL_DICE'; playerId: string; roomId: string }
  | { type: 'ROLL_RESULT'; playerId: string; roomId: string; roll: number[]; poses?: DiePose[]; trajectory?: number[]; stepsPerFrame?: number } // 上报物理动画最终点数与姿态
  | { type: 'KEEP_DICE'; playerId: string; roomId: string; keep: number[] }
  | { type: 'BANK_SCORE'; playerId: string; roomId: string }
  | { type: 'REMATCH_READY'; playerId: string; roomId: string }
  | { type: 'LEAVE_TABLE'; playerId: string; roomId: string }
  | { type: 'CHAT'; playerId: string; roomId: string; text: string }

export interface ServerMessage {
  type: 'GAME_STATE' | 'ROLL_REQUESTED' | 'CHAT' | 'ERROR' | 'ROOM_LEFT'
  state?: GameState
  roll?: number[] // ROLL_REQUESTED 时携带 [骰子数量, 随机种子]
  chat?: ChatMessage
  error?: string
  playerId?: string
  roomId?: string
  timestamp: number
}
