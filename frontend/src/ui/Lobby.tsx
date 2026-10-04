import { useState } from 'react'
import { useGameStore } from '../store'
import { startAmbient } from '../audio'

export function Lobby() {
  const joined = useGameStore((s) => s.state !== null)
  const join = useGameStore((s) => s.join)
  const connected = useGameStore((s) => s.connected)
  const [name, setName] = useState('')
  const [room, setRoom] = useState('tavern')

  if (joined) return null

  const submit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return
    startAmbient()
    join(name.trim(), room.trim() || 'tavern')
  }

  return (
    <div className="lobby">
      <form className="lobby-card parchment" onSubmit={submit}>
        <h1>🎲 酒馆骰子</h1>
        <p className="subtitle">Farkle · 中世纪骰子对局</p>
        <label>
          你的名字
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="例如：亨利"
            maxLength={16}
          />
        </label>
        <label>
          房间号（相同房间号的朋友会进入同一牌桌）
          <input value={room} onChange={(e) => setRoom(e.target.value)} maxLength={24} />
        </label>
        <button className="btn primary big" type="submit" disabled={connected}>
          {connected ? '连接中…' : '进入酒馆'}
        </button>
        <p className="hint">目标：率先攒满 10,000 分 · 首次入库至少 300 分</p>
      </form>
    </div>
  )
}
