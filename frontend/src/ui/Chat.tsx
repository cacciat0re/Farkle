import { useEffect, useRef, useState } from 'react'
import { useGameStore } from '../store'

export function Chat() {
  const state = useGameStore((s) => s.state)
  const messages = useGameStore((s) => s.messages)
  const sendChat = useGameStore((s) => s.sendChat)
  const [text, setText] = useState('')
  const listRef = useRef<HTMLDivElement>(null)

  // 有新消息时自动滚到底部
  useEffect(() => {
    const list = listRef.current
    if (list) list.scrollTop = list.scrollHeight
  }, [messages])

  if (!state) return null

  const submit = () => {
    const value = text.trim()
    if (!value) return
    sendChat(value)
    setText('')
  }

  return (
    <div className="chat-panel">
      <div className="chat-list" ref={listRef}>
        {messages.map((m, i) => (
          <div key={`${m.timestamp}-${i}`} className={`chat-msg ${m.kind}`}>
            {m.kind === 'chat' && <span className="chat-name">{m.name}</span>}
            <span className="chat-text">{m.text}</span>
          </div>
        ))}
      </div>
      <form
        className="chat-input"
        onSubmit={(e) => {
          e.preventDefault()
          submit()
        }}
      >
        <input
          value={text}
          maxLength={200}
          placeholder="输入消息"
          onChange={(e) => setText(e.target.value)}
        />
        <button type="submit" className="chat-send" disabled={text.trim().length === 0}>
          发送
        </button>
      </form>
    </div>
  )
}
