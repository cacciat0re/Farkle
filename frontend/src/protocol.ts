export type Phase = 'waiting' | 'rolling' | 'scoring' | 'turnEnd' | 'gameOver'

export interface Player {
  id: string
  name: string
  score: number
  hasBanked: boolean
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
}

export type ClientMessage =
  | { type: 'JOIN_ROOM'; playerId: string; name: string; roomId: string }
  | { type: 'PLAYER_READY'; playerId: string; roomId: string }
  | { type: 'ROLL_DICE'; playerId: string; roomId: string }
  | { type: 'ROLL_RESULT'; playerId: string; roomId: string; roll: number[] } // 客户端上报物理动画的最终点数
  | { type: 'KEEP_DICE'; playerId: string; roomId: string; keep: number[] }
  | { type: 'BANK_SCORE'; playerId: string; roomId: string }
  | { type: 'REMATCH_READY'; playerId: string; roomId: string }
  | { type: 'LEAVE_TABLE'; playerId: string; roomId: string }

export interface ServerMessage {
  type: 'GAME_STATE' | 'ROLL_REQUESTED' | 'ERROR' | 'ROOM_LEFT'
  state?: GameState
  roll?: number[] // ROLL_REQUESTED 时携带 [骰子数量, 随机种子]
  error?: string
  playerId?: string
  roomId?: string
  timestamp: number
}
