import { create } from 'zustand'
import type { ClientMessage, GameState, ServerMessage } from './protocol'

interface GameStore {
  // 连接信息
  myId: string
  myName: string
  roomId: string
  connected: boolean
  error: string | null

  // 游戏状态（服务端权威：计分/回合/锁定；点数来自本地物理动画）
  state: GameState | null

  // 本地交互状态
  selected: number[] // 玩家点选准备锁定的骰子索引（diceValues 下标）
  rollSeq: number // 每次投掷递增，驱动 3D 场景动画
  rollingCount: number // 本次投掷的骰子数量（ROLL_REQUESTED 时确定）
  rollSeed: number // 本次投掷的种子（全桌一致，保证物理结果一致）
  settledValues: (number | null)[] // 各骰子物理停稳后的实际点数

  // actions
  join: (name: string, roomId: string) => void
  startGame: () => void
  rollDice: () => void
  reportDieSettled: (index: number, value: number) => void
  toggleSelect: (index: number) => void
  clearSelection: () => void
  keepSelected: () => void
  bank: () => void
  dismissError: () => void
}

let ws: WebSocket | null = null

function send(msg: ClientMessage) {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify(msg))
  }
}

export const useGameStore = create<GameStore>((set, get) => ({
  myId: crypto.randomUUID(),
  myName: '',
  roomId: 'tavern',
  connected: false,
  error: null,
  state: null,
  selected: [],
  rollSeq: 0,
  rollingCount: 0,
  rollSeed: 0,
  settledValues: [],

  join: (name, roomId) => {
    const { myId } = get()
    set({ myName: name, roomId, error: null })
    if (ws) ws.close()
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    // vite 开发服务器（5173）直连 8080；其余（含 docker/nginx 生产）走同源 /ws 反代
    const base =
      location.port === '5173'
        ? `${proto}://${location.hostname}:8080`
        : `${proto}://${location.host}`
    ws = new WebSocket(`${base}/ws?room=${encodeURIComponent(roomId)}`)

    ws.onopen = () => {
      set({ connected: true })
      send({ type: 'JOIN_ROOM', playerId: myId, name, roomId })
    }
    ws.onclose = () => set({ connected: false })
    ws.onerror = () => set({ connected: false })
    ws.onmessage = (ev) => {
      const msg = JSON.parse(ev.data as string) as ServerMessage
      if (msg.type === 'ERROR') {
        set({ error: msg.error ?? '未知错误' })
      } else if (msg.type === 'ROLL_REQUESTED') {
        // 服务端请求投掷：roll 字段为 [骰子数量, 种子]
        const count = msg.roll?.[0] ?? 6
        const seed = msg.roll?.[1] ?? 0
        set((s) => ({
          state: msg.state ?? s.state,
          rollingCount: count,
          rollSeed: seed,
          settledValues: new Array(count).fill(null),
          rollSeq: s.rollSeq + 1, // 驱动 3D 物理投掷
          selected: [],
        }))
      } else if (msg.type === 'GAME_STATE') {
        if (msg.state) {
          const me = get().myId
          const myTurn = msg.state.players[msg.state.currentTurn]?.id === me
          set((s) => ({
            state: msg.state!,
            selected: myTurn ? s.selected : [],
          }))
        }
      }
    }
  },

  startGame: () => {
    const { myId, roomId } = get()
    send({ type: 'START_GAME', playerId: myId, roomId })
  },

  rollDice: () => {
    const { myId, roomId } = get()
    send({ type: 'ROLL_DICE', playerId: myId, roomId })
    // 注意：rollSeq 不在这里加，等服务端 ROLL_REQUESTED 广播后统一加，
    // 保证同桌所有人（包括观战者）同步播放投掷动画
  },

  reportDieSettled: (index, value) => {
    const { settledValues, state, myId, roomId } = get()
    if (!state) return
    const next = settledValues.slice()
    next[index] = value
    set({ settledValues: next })
    // 只有投掷者需要上报服务端；全部停稳 → 上报物理结果
    const isRoller = state.players[state.currentTurn]?.id === myId
    if (isRoller && state.phase === 'rolling' && next.every((v) => v !== null)) {
      send({ type: 'ROLL_RESULT', playerId: myId, roomId, roll: next as number[] })
    }
  },

  toggleSelect: (index) => {
    set((s) => {
      const idx = s.selected.indexOf(index)
      if (idx >= 0) {
        return { selected: s.selected.filter((i) => i !== index) }
      }
      return { selected: [...s.selected, index] }
    })
  },

  clearSelection: () => set({ selected: [] }),

  keepSelected: () => {
    const { myId, roomId, selected, settledValues } = get()
    if (selected.length === 0) return
    // 将索引映射为物理骰子的实际值发给服务端
    const keep = selected
      .map((i) => settledValues[i])
      .filter((v) => v !== null && v !== undefined)
    if (keep.length === 0) return
    send({ type: 'KEEP_DICE', playerId: myId, roomId, keep })
    set({ selected: [] })
  },

  bank: () => {
    const { myId, roomId } = get()
    send({ type: 'BANK_SCORE', playerId: myId, roomId })
  },

  dismissError: () => set({ error: null }),
}))

/** 便捷选择器 */
export const useIsMyTurn = () =>
  useGameStore((s) => s.state?.players[s.state.currentTurn]?.id === s.myId)
