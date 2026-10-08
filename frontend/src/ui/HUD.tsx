import { useEffect, useState } from 'react'
import { useGameStore, useIsMyTurn } from '../store'
import { MIN_BANK_SCORE, TARGET_SCORE, scoreFull } from '../game/scoring'
import { Chat } from './Chat'

export function HUD() {
  const state = useGameStore((s) => s.state)
  const connected = useGameStore((s) => s.connected)
  const selected = useGameStore((s) => s.selected)
  const error = useGameStore((s) => s.error)
  const isMyTurn = useIsMyTurn()
  const playerReady = useGameStore((s) => s.playerReady)
  const keepAndRoll = useGameStore((s) => s.keepAndRoll)
  const keepAndBank = useGameStore((s) => s.keepAndBank)
  const rematchReady = useGameStore((s) => s.rematchReady)
  const leaveTable = useGameStore((s) => s.leaveTable)
  const joinedAt = useGameStore((s) => s.joinedAt)
  const leaving = useGameStore((s) => s.leaving)
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
  // 对手的选择同样通过服务端广播过来，记分板两边都能看到
  const shownSelection = state.selection ?? []
  const shownValues = shownSelection
    .map((i) => settledValues[i])
    .filter((v): v is number => v !== null && v !== undefined)
  const [shownScore] = scoreFull(shownValues)
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
  // 首次入库有 300 分门槛：没到门槛就别让按钮点下去，避免服务端拒绝
  const projectedTurnScore = state.turnScore + (hasSelection && selectedIsValid ? selectedScore : 0)
  const canBank = currentPlayer?.hasBanked || projectedTurnScore >= MIN_BANK_SCORE
  // 骰子滚动中不显示底部操作区；只有轮到自己选骰、等待、对局结束或等待对手时才有内容
  const showActions =
    state.phase !== 'rolling' && (state.phase === 'waiting' || state.phase === 'gameOver' || scoring || !isMyTurn)

  return (
    <div className="hud">
      <Chat />

      {/* 顶部：玩家计分板 */}
      <div className="scoreboard">
        <div className="score-goal">目标 {TARGET_SCORE.toLocaleString()}</div>
        <div className="score-columns">
          {state.players.map((p, i) => {
            const active = i === state.currentTurn
            return (
              <div
                key={p.id}
                className={`score-col ${active ? 'active' : ''} ${p.id === myId ? 'me' : ''}`}
              >
                <div className="score-name">{p.name}</div>
                <div className="score-row">
                  <span>总分</span>
                  <b>{p.score.toLocaleString()}</b>
                </div>
                <div className="score-row">
                  <span>本轮</span>
                  <b>{active ? state.turnScore.toLocaleString() : 0}</b>
                </div>
                <div className="score-row">
                  <span>选择</span>
                  <b>{active ? shownScore.toLocaleString() : 0}</b>
                </div>
              </div>
            )
          })}
        </div>
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

      {/* 底部操作区：滚动中或没有可做的事时整块隐藏，避免空框残留 */}
      {showActions && (
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
              <button
                className="btn primary"
                disabled={!hasSelection || !selectedIsValid}
                onClick={keepAndRoll}
              >
                选择并投掷{hasSelection && selectedIsValid ? `（+${selectedScore}）` : ''}
              </button>
              <button
                className="btn gold"
                disabled={!hasSelection || !selectedIsValid || !canBank}
                onClick={keepAndBank}
              >
                选择并结束回合{hasSelection && selectedIsValid ? `（+${selectedScore}）` : ''}
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
      )}

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
