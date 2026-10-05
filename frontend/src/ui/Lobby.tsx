import { useEffect, useState } from 'react'
import { useGameStore } from '../store'

interface TableStatus {
  id: string
  name: string
  maxPlayers: number
  players: number
  inGame: boolean
}

interface LobbyMessage {
  type: 'TABLE_STATUS'
  tables: TableStatus[]
}

export function Lobby() {
  const state = useGameStore((s) => s.state)
  const myName = useGameStore((s) => s.myName)
  const error = useGameStore((s) => s.error)
  const dismissError = useGameStore((s) => s.dismissError)
  const join = useGameStore((s) => s.join)
  const joiningTableId = useGameStore((s) => s.joiningTableId)
  const [tables, setTables] = useState<TableStatus[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (state) return

    let closed = false
    let retry = 0
    let reconnectTimer: number | null = null
    let socket: WebSocket | null = null

    const connect = () => {
      const proto = location.protocol === 'https:' ? 'wss' : 'ws'
      socket = new WebSocket(`${proto}://${location.host}/ws/lobby`)
      socket.onopen = () => {
        retry = 0
      }
      socket.onmessage = (event) => {
        const message = JSON.parse(event.data as string) as LobbyMessage
        if (message.type === 'TABLE_STATUS') {
          setTables(message.tables)
          setLoading(false)
        }
      }
      socket.onclose = () => {
        if (closed) return
        const delay = Math.min(1000 * 2 ** retry, 10000)
        retry += 1
        reconnectTimer = window.setTimeout(connect, delay)
      }
    }

    connect()
    return () => {
      closed = true
      if (reconnectTimer !== null) window.clearTimeout(reconnectTimer)
      socket?.close()
    }
  }, [state])

  if (state) return null

  const busyId = error ? null : joiningTableId

  const chooseTable = (table: TableStatus) => {
    join(table.id)
  }

  return (
    <div className="lobby">
      <section className="lobby-card parchment">
        <header className="lobby-header">
          <h1>Farkle</h1>
          <div className="guest-badge">
            <span>游客</span>
            <strong>{myName}</strong>
          </div>
        </header>

        <div className="lobby-heading">
          <h2>选择牌桌</h2>
          <span>{loading ? '加载中…' : `${tables.length} 张牌桌`}</span>
        </div>

        <div className="table-grid">
          {tables.map((table) => {
            const full = table.players >= table.maxPlayers
            const disabled = table.inGame || full || busyId !== null
            const status = table.inGame ? '对局中' : full ? '已满' : '等待中'
            return (
              <button
                key={table.id}
                type="button"
                className={`table-card ${busyId === table.id ? 'active' : ''}`}
                disabled={disabled}
                onClick={() => chooseTable(table)}
              >
                <span className="table-name">{table.name}</span>
                <span className="table-status">
                  {table.players}/{table.maxPlayers} · {status}
                </span>
                <span className="table-action">
                  {busyId === table.id ? '连接中…' : '入座'}
                </span>
              </button>
            )
          })}
        </div>

        <p className="hint">目标：率先攒满 3,000 分 · 首次入库至少 300 分</p>
      </section>

      {error && (
        <div
          className="toast error"
          onClick={() => {
            dismissError()
          }}
        >
          {error}
        </div>
      )}
    </div>
  )
}
