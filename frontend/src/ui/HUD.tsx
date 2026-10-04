import { useGameStore, useIsMyTurn } from '../store'
import { scoreDice } from '../game/scoring'

export function HUD() {
  const state = useGameStore((s) => s.state)
  const selected = useGameStore((s) => s.selected)
  const error = useGameStore((s) => s.error)
  const isMyTurn = useIsMyTurn()
  const startGame = useGameStore((s) => s.startGame)
  const rollDice = useGameStore((s) => s.rollDice)
  const keepSelected = useGameStore((s) => s.keepSelected)
  const bank = useGameStore((s) => s.bank)
  const clearSelection = useGameStore((s) => s.clearSelection)
  const dismissError = useGameStore((s) => s.dismissError)

  if (!state) return null

  // selected 是骰子索引，映射为值再算分
  const settledValues = useGameStore((s) => s.settledValues)
  const selectedValues = selected
    .map((i) => settledValues[i])
    .filter((v) => v !== null && v !== undefined)
  const selScore = selectedValues.length > 0 ? scoreDice(selectedValues as number[]) : null
  const currentPlayer = state.players[state.currentTurn]
  const animating = state.animatingTill > Date.now()
  const scoring = state.phase === 'scoring' && isMyTurn

  return (
    <div className="hud">
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
            {animating ? '⏳ 骰子滚动中…' : `${currentPlayer?.name ?? ''} 的回合`}
          </div>
        )}
        {state.phase === 'turnEnd' && state.farkled && (
          <div className="banner farkle">💀 FARKLE！本回合得分清零</div>
        )}
        {state.phase === 'turnEnd' && !state.farkled && state.lastBanked > 0 && (
          <div className="banner small">💰 入库 +{state.lastBanked} 分</div>
        )}
        {state.phase === 'gameOver' && (
          <div className="banner victory">🏆 {state.winner} 赢得对局！</div>
        )}
      </div>

      {/* 底部操作区 */}
      <div className="actions parchment">
        {state.phase === 'waiting' && (
          <button className="btn primary" onClick={startGame}>
            ⚔️ 开始对局
          </button>
        )}

        {scoring && (
          <>
            <div className="turn-score">
              回合分 <b>{state.turnScore}</b>
              {selScore && !selScore.isFarkle && <span className="sel-preview"> +{selScore.baseScore}</span>}
            </div>
            {selected.length > 0 && (
              <button className="btn" onClick={clearSelection}>
                取消选择
              </button>
            )}
            <button
              className="btn primary"
              disabled={!selScore || selScore.isFarkle}
              onClick={keepSelected}
            >
              🔒 锁定所选{selScore && !selScore.isFarkle ? `（+${selScore.baseScore}）` : ''}
            </button>
            <button className="btn" onClick={rollDice}>
              🎲 继续掷骰
            </button>
            <button className="btn gold" onClick={bank}>
              💰 入库（{state.turnScore}）
            </button>
          </>
        )}

        {state.phase !== 'waiting' && !isMyTurn && state.phase !== 'gameOver' && (
          <div className="wait">等待 {currentPlayer?.name} 行动…</div>
        )}
      </div>

      {error && (
        <div className="toast error" onClick={dismissError}>
          ⚠️ {error}
        </div>
      )}
    </div>
  )
}
