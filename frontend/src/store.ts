import { create } from 'zustand'
import type { ChatMessage, ClientMessage, DiePose, GameState, ServerMessage } from './protocol'
import { selectableIndices } from './game/scoring'
import { precomputeRoll, syntheticRoll } from './three/trajectory'

// 与 Scene 中的托盘边界一致
const TRAY_BOUNDS = { x: 4.2, z: 2.6 }

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

  // 游戏状态（服务端权威：计分/回合/锁定；点数来自本地物理动画）
  state: GameState | null
  messages: ChatMessage[] // 桌内聊天与系统事件（加入/离开）

  // 本地交互状态
  selected: number[] // 玩家点选准备锁定的骰子索引（diceValues 下标）
  rollSeq: number // 每次投掷递增，驱动 3D 场景动画
  rollingCount: number // 本次投掷的骰子数量（ROLL_REQUESTED 时确定）
  rollSeed: number // 本次投掷的种子（全桌一致，保证物理结果一致）
  settledValues: (number | null)[] // 各骰子物理停稳后的实际点数
  settledPoses: (DiePose | null)[] // 各骰子物理停稳后的实际姿态（投掷者上报给服务端）

  // 本轮投掷的可视化轨迹：投掷者离线预演一次，全桌（含投掷者）回放同一份
  trajectory: number[] | null
  trajectoryDice: number // 轨迹里每帧包含几颗骰子
  trajectoryRollId: number // 轨迹属于哪一次投掷
  trajectoryLive: boolean // 本端在投掷开始时就已在线（才从头播动画）
  stepsPerFrame: number

  // actions
  join: (tableId: string) => void
  leaveTable: () => void
  playerReady: () => void
  rollDice: () => void
  reportDieSettled: (index: number, value: number, pose: DiePose) => void
  toggleSelect: (index: number) => void
  clearSelection: () => void
  keepSelected: () => void
  bank: () => void
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
  rollSeq: 0,
  rollSeed: 0,
  settledValues: [],
  settledPoses: [],
  trajectory: null,
  trajectoryDice: 0,
  trajectoryRollId: 0,
  trajectoryLive: false,
  stepsPerFrame: 3,
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
  rollSeq: 0,
  rollingCount: 0,
  rollSeed: 0,
  settledValues: [],
  settledPoses: [],
  trajectory: null,
  trajectoryDice: 0,
  trajectoryRollId: 0,
  trajectoryLive: false,
  stepsPerFrame: 3,

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
          rollSeed: seed,
          settledValues: new Array(count).fill(null),
          settledPoses: new Array(count).fill(null),
          rollSeq: s.rollSeq + 1, // 驱动 3D 物理投掷
          selected: [],
          // 新一轮开始：投掷者的轨迹还没算完，先清空上一轮
          trajectory: null,
          trajectoryDice: count,
          trajectoryRollId: nextState?.rollId ?? 0,
          trajectoryLive: true,
          stepsPerFrame: 3,
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
          void precomputeRoll(seed, count, TRAY_BOUNDS)
            .then((res) => report(res.values, res.frames, res.stepsPerFrame))
            .catch((err) => {
              // 预演失败绝不能让回合卡死：退化成纯数学的兜底动画（点数与画面仍然自洽）
              console.error('[farkle] precomputeRoll failed, falling back:', err)
              const fallback = syntheticRoll(seed, count, TRAY_BOUNDS)
              report(fallback.values, fallback.frames, fallback.stepsPerFrame)
            })
        }
      } else if (msg.type === 'GAME_STATE') {
        if (msg.state) {
          const incoming = msg.state
          const me = get().myId
          const myTurn = incoming.players[incoming.currentTurn]?.id === me
          set((s) => {
            const sameRoll = s.trajectoryRollId === incoming.rollId && incoming.rollId !== 0
            const traj = incoming.trajectory && incoming.trajectory.length > 0 ? incoming.trajectory : null
            // 同一轮里必须沿用同一个数组引用，否则每次状态广播都会重播一遍投掷动画
            const adopt = traj !== null && (s.trajectory === null || !sameRoll)
            if (adopt) {
              const dice = incoming.trajectoryDice ?? (s.rollingCount || 6)
              console.log(
                `[farkle] 回放轨迹 rollId=${incoming.rollId} 骰子=${dice} 帧数=${
                  traj.length / (dice * 7)
                } 实时播放=${sameRoll}`,
              )
            }
            return {
              state: incoming,
              trajectory: adopt ? traj : sameRoll ? s.trajectory : null,
              trajectoryDice: adopt
                ? incoming.trajectoryDice ?? (s.rollingCount || 6)
                : sameRoll
                  ? s.trajectoryDice
                  : s.rollingCount || 6,
              trajectoryRollId: incoming.rollId,
              // 只有本轮开局就在线的客户端才从头播；补看的人直接停在末帧
              trajectoryLive: adopt ? sameRoll : sameRoll ? s.trajectoryLive : false,
              stepsPerFrame: adopt
                ? incoming.stepsPerFrame ?? 3
                : sameRoll
                  ? s.stepsPerFrame
                  : 3,
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

  rollDice: () => {
    const { myId, roomId } = get()
    send({ type: 'ROLL_DICE', playerId: myId, roomId })
    // 注意：rollSeq 不在这里加，等服务端 ROLL_REQUESTED 广播后统一加，
    // 保证同桌所有人（包括观战者）同步播放投掷动画
  },

  reportDieSettled: (index, value, pose) => {
    const { settledValues, settledPoses, state, myId, roomId } = get()
    if (!state) return
    const next = settledValues.slice()
    next[index] = value
    const nextPoses = settledPoses.slice()
    nextPoses[index] = pose
    set({ settledValues: next, settledPoses: nextPoses })
    // 只有投掷者需要上报服务端；全部停稳 → 上报物理结果
    const isRoller = state.players[state.currentTurn]?.id === myId
    if (isRoller && state.phase === 'rolling' && next.every((v) => v !== null)) {
      send({
        type: 'ROLL_RESULT',
        playerId: myId,
        roomId,
        roll: next as number[],
        // 权威姿态：所有客户端据此对齐到完全相同的停靠画面
        poses: nextPoses.every((p) => p !== null) ? (nextPoses as DiePose[]) : undefined,
      })
    }
  },

  toggleSelect: (index) => {
    set((s) => {
      const state = s.state
      if (!state || state.phase !== 'scoring') return {}
      if (state.players[state.currentTurn]?.id !== s.myId) return {}
      if (s.settledValues[index] == null) return {}

      const idx = s.selected.indexOf(index)
      if (idx >= 0) {
        return { selected: s.selected.filter((i) => i !== index) }
      }

      const allowed = selectableIndices(s.settledValues, s.selected)
      if (!allowed.has(index)) return {}
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
