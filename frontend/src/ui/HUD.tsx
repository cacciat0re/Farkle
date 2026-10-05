import { useEffect, useState } from 'react'
import { useGameStore, useIsMyTurn } from '../store'
import { scoreFull } from '../game/scoring'
import { Chat } from './Chat'

export function HUD() {
  const state = useGameStore((s) => s.state)
  const connected = useGameStore((s) => s.connected)
  const selected = useGameStore((s) => s.selected)
  const error = useGameStore((s) => s.error)
  const isMyTurn = useIsMyTurn()
  const playerReady = useGameStore((s) => s.playerReady)
  const rollDice = useGameStore((s) => s.rollDice)
  const keepSelected = useGameStore((s) => s.keepSelected)
  const bank = useGameStore((s) => s.bank)
  const rematchReady = useGameStore((s) => s.rematchReady)
  const leaveTable = useGameStore((s) => s.leaveTable)
  const joinedAt = useGameStore((s) => s.joinedAt)
  const leaving = useGameStore((s) => s.leaving)
  const clearSelection = useGameStore((s) => s.clearSelection)
  const dismissError = useGameStore((s) => s.dismissError)
  const settledValues = useGameStore((s) => s.settledValues)
  const [clock, setClock] = useState(() => Date.now())

  useEffect(() => {
    if (state?.phase !== 'waiting') return
    const timer = window.setInterval(() => setClock(Date.now()), 250)
    return () => window.clearInterval(timer)
  }, [state?.phase])

  if (!state) return null

  // selected 是骰子索引，映射为值再算分
  const selectedValues = selected
    .map((i) => settledValues[i])
    .filter((v) => v !== null && v !== undefined)
  const [selectedScore, selectedIsValid] = scoreFull(selectedValues as number[])
  const hasSelection = selectedValues.length > 0
  const currentPlayer = state.players[state.currentTurn]
  const animating = state.animatingTill > clock
  const scoring = state.phase === 'scoring' && isMyTurn
  const myId = useGameStore.getState().myId
  const iAmReady = state.rematchReady?.includes(myId) ?? false
  const opponentReadyForRematch = (state.rematchReady?.length ?? 0) > 0 && !iAmReady
  const iAmReadyToStart = state.readyPlayers?.includes(myId) ?? false
  const readyCount = state.readyPlayers?.length ?? 0
  const leaveElapsed = joinedAt === null ? 0 : clock - joinedAt
  const canLeave = leaveElapsed >= 10000
  const leaveSeconds = Math.max(0, 10 - Math.floor(leaveElapsed / 1000))

  return (
    <div className="hud">
      <Chat />

      {/* 顶部：玩家计分板 */}
      <div className="scoreboard parchment">
        {state.players.map((p, i) => (
          <div
            key={p.id}
            className={`player-chip ${i === state.currentTurn ? 'active' : ''} ${
              p.id === useGameStore.getState().myId ? 'me' : ''
            }`}
          >
            <span className="pname">{p.name}</span>
            <span className="pscore">{p.score.toLocaleString()}</span>
          </div>
        ))}
      </div>

      {/* 回合信息 */}
      <div className="turn-info">
        {state.phase === 'rolling' && (
          <div className="banner small">
            {animating ? '骰子滚动中…' : `${currentPlayer?.name ?? ''} 的回合`}
          </div>
        )}
        {state.phase === 'turnEnd' && state.farkled && (
          <div className="banner farkle">FARKLE！本回合得分清零</div>
        )}
        {state.phase === 'turnEnd' && !state.farkled && state.lastBanked > 0 && (
          <div className="banner small">入库 +{state.lastBanked} 分</div>
        )}
        {state.phase === 'gameOver' && (
          <div className="banner victory">{state.winner} 赢得对局</div>
        )}
      </div>

      {/* 底部操作区 */}
      <div className="actions parchment">
        {state.phase === 'waiting' && (
          <>
            <div className="wait">已准备 {readyCount}/{state.players.length}</div>
            <button className="btn primary" onClick={playerReady} disabled={iAmReadyToStart}>
              {iAmReadyToStart ? '已准备，等待对手' : '准备'}
            </button>
            <button className="btn" onClick={leaveTable} disabled={!canLeave || leaving}>
              {leaving ? '正在退出...' : canLeave ? '退出房间' : `退出房间（${leaveSeconds}s）`}
            </button>
          </>
        )}

        {scoring && (
          <>
            <div className="turn-score">
              回合分 <b>{state.turnScore}</b>
              {hasSelection && selectedIsValid && <span className="sel-preview"> +{selectedScore}</span>}
            </div>
            {selected.length > 0 && (
              <button className="btn" onClick={clearSelection}>
                取消选择
              </button>
            )}
            <button
              className="btn primary"
              disabled={!hasSelection || !selectedIsValid}
              onClick={keepSelected}
            >
              锁定所选{hasSelection && selectedIsValid ? `（+${selectedScore}）` : ''}
            </button>
            {/* 必须先锁定本轮的得分骰；有未锁定的选择时也不允许直接重掷 */}
            <button
              className="btn"
              disabled={!state.keptThisRoll || selected.length > 0}
              onClick={rollDice}
            >
              继续掷骰
            </button>
            <button className="btn gold" disabled={!state.keptThisRoll} onClick={bank}>
              入库（{state.turnScore}）
            </button>
          </>
        )}

        {state.phase === 'gameOver' && (
          <>
            <button className="btn primary" onClick={rematchReady} disabled={iAmReady}>
              {iAmReady ? '等待对手选择' : opponentReadyForRematch ? '继续游戏（对手已选择）' : '继续游戏'}
            </button>
            <button className="btn" onClick={leaveTable}>
              退出到大厅
            </button>
          </>
        )}

        {state.phase !== 'waiting' && !isMyTurn && state.phase !== 'gameOver' && (
          <div className="wait">等待 {currentPlayer?.name} 行动…</div>
        )}
      </div>

      {error && (
        <div className="toast error" onClick={dismissError}>
          {error}
        </div>
      )}

      {/* 连接被意外断开时必须明说，否则玩家会一直卡在一个不会更新的旧状态里 */}
      {state && !connected && <div className="toast disconnected">与牌桌的连接已断开</div>}
    </div>
  )
}
