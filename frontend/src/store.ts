import { create } from 'zustand'
import type { ChatMessage, ClientMessage, GameState, ServerMessage } from './protocol'
import { selectableIndices } from './game/scoring'
import {
  emptyRollView,
  rollViewFromState,
  rollViewOnRollRequested,
  type RollView,
} from './game/rollView'
import { TRAY } from './game/layout'
import { precomputeRoll, syntheticRoll } from './three/trajectory'

interface GameStore {
  // 连接信息
  myId: string
  myName: string
  roomId: string
  connected: boolean
  joiningTableId: string | null
  joinedAt: number | null
  leaving: boolean
  error: string | null

  // 游戏状态（服务端权威：计分/回合/锁定/点数）
  state: GameState | null
  messages: ChatMessage[] // 桌内聊天与系统事件（加入/离开）

  // 本地交互状态
  selected: number[] // 玩家点选准备锁定的骰子索引（diceValues 下标）
  rollingCount: number // 本次投掷的骰子数量（ROLL_REQUESTED 时确定）
  settledValues: (number | null)[] // 本轮各骰子的点数（服务端权威）

  // 本轮投掷的可视化轨迹（投掷者预演一次，全桌回放同一份）
  roll: RollView

  // actions
  join: (tableId: string) => void
  leaveTable: () => void
  playerReady: () => void
  toggleSelect: (index: number) => void
  keepAndRoll: () => void // 选择并投掷
  keepAndBank: () => void // 选择并结束回合
  rematchReady: () => void
  sendChat: (text: string) => void
  dismissError: () => void
}

let ws: WebSocket | null = null

function createGuest() {
  const id = crypto.randomUUID()
  const suffix = id.replace(/-/g, '').slice(0, 4).toUpperCase()
  return { id, name: `traveler${suffix}` }
}

const guest = createGuest()

const emptyRoomState = {
  state: null,
  messages: [],
  roomId: '',
  connected: false,
  joiningTableId: null,
  joinedAt: null,
  leaving: false,
  selected: [],
  rollingCount: 0,
  settledValues: [],
  roll: emptyRollView,
  error: null,
}

function send(msg: ClientMessage) {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify(msg))
  }
}

export const useGameStore = create<GameStore>((set, get) => ({
  myId: guest.id,
  myName: guest.name,
  roomId: '',
  connected: false,
  joiningTableId: null,
  joinedAt: null,
  leaving: false,
  error: null,
  state: null,
  messages: [],
  selected: [],
  rollingCount: 0,
  settledValues: [],
  roll: emptyRollView,

  join: (tableId) => {
    const { myId, myName } = get()
    if (!tableId) return
    set({
      roomId: tableId,
      joiningTableId: tableId,
      joinedAt: Date.now(),
      leaving: false,
      error: null,
    })
    if (ws) ws.close()
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    // Vite 开发服务器和生产 nginx 都反代 /api 与 /ws，浏览器侧始终使用同源地址
    ws = new WebSocket(`${proto}://${location.host}/ws/${encodeURIComponent(tableId)}`)

    ws.onopen = () => {
      set({ connected: true })
      send({ type: 'JOIN_ROOM', playerId: myId, name: myName, roomId: tableId })
    }
    ws.onclose = () => {
      if (get().leaving) {
        ws = null
        set(emptyRoomState)
      } else {
        set((s) => ({
          connected: false,
          joiningTableId: s.state ? s.joiningTableId : null,
        }))
      }
    }
    ws.onerror = () => set({ connected: false, joiningTableId: null, error: '无法连接到牌桌服务' })
    ws.onmessage = (ev) => {
      const msg = JSON.parse(ev.data as string) as ServerMessage
      if (msg.type === 'ERROR') {
        set({ error: msg.error ?? '未知错误', joiningTableId: null, leaving: false })
      } else if (msg.type === 'ROOM_LEFT') {
        ws = null
        set(emptyRoomState)
      } else if (msg.type === 'ROLL_REQUESTED') {
        // 服务端请求投掷：roll 字段为 [骰子数量, 种子]
        const count = msg.roll?.[0] ?? 6
        const seed = msg.roll?.[1] ?? 0
        const nextState = msg.state ?? get().state
        const isRoller = nextState?.players[nextState.currentTurn]?.id === get().myId
        set((s) => ({
          state: nextState ?? s.state,
          rollingCount: count,
          settledValues: new Array(count).fill(null),
          selected: [],
          // 新一轮开始：投掷者的轨迹还没算完，先清空上一轮
          roll: rollViewOnRollRequested(nextState?.rollId ?? 0, count),
        }))
        // 投掷者离线预演整段轨迹后立刻上报；所有人（含投掷者）回放同一份轨迹
        if (isRoller) {
          const report = (values: number[], trajectory?: number[], stepsPerFrame?: number) =>
            send({
              type: 'ROLL_RESULT',
              playerId: get().myId,
              roomId: get().roomId,
              roll: values,
              trajectory,
              stepsPerFrame,
            })
          void precomputeRoll(seed, count, TRAY)
            .then((res) => report(res.values, res.frames, res.stepsPerFrame))
            .catch((err) => {
              // 预演失败绝不能让回合卡死：退化成纯数学的兜底动画（点数与画面仍然自洽）
              console.error('[farkle] precomputeRoll failed, falling back:', err)
              const fallback = syntheticRoll(seed, count, TRAY)
              report(fallback.values, fallback.frames, fallback.stepsPerFrame)
            })
        }
      } else if (msg.type === 'GAME_STATE') {
        if (msg.state) {
          const incoming = msg.state
          const me = get().myId
          const myTurn = incoming.players[incoming.currentTurn]?.id === me
          set((s) => {
            const roll = rollViewFromState(s.roll, incoming, s.rollingCount)
            // 只有真的换了一份轨迹才打日志（同一轮沿用同一个引用）
            if (roll.trajectory && roll.trajectory !== s.roll.trajectory) {
              const dice = roll.trajectoryDice || 6
              console.log(
                `[farkle] 回放轨迹 rollId=${incoming.rollId} 骰子=${dice} 帧数=${
                  roll.trajectory.length / (dice * 7)
                } 实时播放=${roll.trajectoryLive}`,
              )
            }
            return {
              state: incoming,
              roll,
              joiningTableId: null,
              selected: myTurn ? s.selected : [],
              // 骰子位姿来自轨迹回放，本端不再跑物理；散骰点数直接采用服务端权威结果
              settledValues:
                incoming.diceValues.length === s.rollingCount ? incoming.diceValues : s.settledValues,
            }
          })
        }
      } else if (msg.type === 'CHAT') {
        if (msg.chat) {
          // 只保留最近 100 条，避免长局内存无限增长
          set((s) => ({ messages: [...s.messages, msg.chat!].slice(-100) }))
        }
      } else if (msg.type === 'SELECTION') {
        // 当前玩家的选择变化（可能是自己，也可能是对手）：只更新用于显示的 selection
        set((s) => {
          if (!s.state) return {}
          if (msg.rollId != null && msg.rollId !== s.state.rollId) return {}
          return { state: { ...s.state, selection: msg.selection ?? [] } }
        })
      }
    }
  },

  leaveTable: () => {
    const { state, myId, roomId, leaving, connected } = get()
    // 连接已经断了就别再等服务端确认，直接回大厅，否则按钮会永远停在"正在退出"
    if (!connected) {
      if (ws) ws.close()
      ws = null
      set(emptyRoomState)
      return
    }
    if (state?.phase === 'waiting' && !leaving) {
      send({ type: 'LEAVE_TABLE', playerId: myId, roomId })
      set({ leaving: true })
      return
    }
    if (ws) ws.close()
    ws = null
    set(emptyRoomState)
  },

  playerReady: () => {
    const { myId, roomId } = get()
    send({ type: 'PLAYER_READY', playerId: myId, roomId })
  },

  toggleSelect: (index) => {
    const { state, myId, settledValues, selected } = get()
    if (!state || state.phase !== 'scoring') return
    if (state.players[state.currentTurn]?.id !== myId) return
    if (settledValues[index] == null) return

    let next: number[]
    if (selected.includes(index)) {
      next = selected.filter((i) => i !== index)
    } else {
      if (!selectableIndices(settledValues, selected).has(index)) return
      next = [...selected, index]
    }
    // 本地立即生效，同时广播出去让对手也能看到选中的是哪几颗
    set({ selected: next, state: { ...state, selection: next } })
    send({ type: 'SELECT_DICE', playerId: myId, roomId: get().roomId, select: next })
  },

  keepAndRoll: () => {
    const { myId, roomId, selected, settledValues } = get()
    // 将索引映射为物理骰子的实际值发给服务端
    const keep = selected
      .map((i) => settledValues[i])
      .filter((v): v is number => v !== null && v !== undefined)
    if (keep.length === 0) return
    send({ type: 'KEEP_AND_ROLL', playerId: myId, roomId, keep })
    set((s) => ({ selected: [], state: s.state ? { ...s.state, selection: [] } : null }))
  },

  keepAndBank: () => {
    const { myId, roomId, selected, settledValues } = get()
    const keep = selected
      .map((i) => settledValues[i])
      .filter((v): v is number => v !== null && v !== undefined)
    if (keep.length === 0) return
    send({ type: 'KEEP_AND_BANK', playerId: myId, roomId, keep })
    set((s) => ({ selected: [], state: s.state ? { ...s.state, selection: [] } : null }))
  },

  rematchReady: () => {
    const { myId, roomId } = get()
    send({ type: 'REMATCH_READY', playerId: myId, roomId })
  },

  sendChat: (text) => {
    const { myId, roomId, state } = get()
    const value = text.trim()
    if (!state || !value) return
    send({ type: 'CHAT', playerId: myId, roomId, text: value.slice(0, 200) })
  },

  dismissError: () => set({ error: null }),
}))

/** 便捷选择器 */
export const useIsMyTurn = () =>
  useGameStore((s) => s.state?.players[s.state.currentTurn]?.id === s.myId)

// 开发期把 store 挂到 window，方便在浏览器 console 里直接查看状态与轨迹
// （例如 __farkle.getState().trajectory）
if (import.meta.env.DEV) {
  ;(window as unknown as { __farkle?: typeof useGameStore }).__farkle = useGameStore
}
