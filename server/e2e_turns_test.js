// 多回合顺序稳定性测试（新协议）：node e2e_turns_test.js
const WebSocket = require('ws')
const room = 'butter'
const url = `ws://localhost:8080/ws/${room}`
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))
const send = (c, msg) => c.ws.send(JSON.stringify(msg))

// 模拟物理客户端：收到 ROLL_REQUESTED 后延迟上报随机点数
function autoRoll(c) {
  c.ws.on('message', (d) => {
    const m = JSON.parse(d)
    if (m.type === 'ROLL_REQUESTED' && c.state?.players[c.state.currentTurn]?.id === c.pid) {
      const count = m.roll[0]
      const values = Array.from({ length: count }, () => 1 + Math.floor(Math.random() * 6))
      setTimeout(() => send(c, { type: 'ROLL_RESULT', playerId: c.pid, roomId: room, roll: values }), 200)
    }
  })
}

function client(name, pid) {
  const ws = new WebSocket(url)
  const c = { ws, name, pid, state: null }
  ws.on('message', (d) => {
    const m = JSON.parse(d)
    if (m.type === 'GAME_STATE') c.state = m.state
    if (m.type === 'ROLL_REQUESTED') c.state = m.state
  })
  autoRoll(c)
  return new Promise((res) => ws.on('open', () => res(c)))
}

async function main() {
  const a = await client('Alice', 'a')
  const b = await client('Bob', 'b')
  send(a, { type: 'JOIN_ROOM', playerId: 'a', name: 'Alice', roomId: room })
  send(b, { type: 'JOIN_ROOM', playerId: 'b', name: 'Bob', roomId: room })
  await sleep(300)

  const orderAtJoin = a.state.players.map((p) => p.id).join(',')
  console.log('加入顺序:', orderAtJoin)

  send(a, { type: 'PLAYER_READY', playerId: 'a', roomId: room })
  send(b, { type: 'PLAYER_READY', playerId: 'b', roomId: room })
  await sleep(800)

  const turnHistory = []

  // 连续打 8 个回合动作
  for (let round = 0; round < 8; round++) {
    if (!a.state || a.state.phase === 'gameOver') break
    const cur = a.state.players[a.state.currentTurn]
    if (a.state.phase === 'scoring') {
      turnHistory.push(`${cur.id}(scoring)`)
      const me = cur.id === 'a' ? a : b
      // 锁定 1 或 5（如果有）
      const keep = me.state.diceValues.filter((v) => v === 1 || v === 5)
      if (keep.length > 0) {
        send(me, { type: 'KEEP_DICE', playerId: cur.id, roomId: room, keep })
        await sleep(250)
      }
      if (me.state.turnScore >= 300) {
        send(me, { type: 'BANK_SCORE', playerId: cur.id, roomId: room })
        await sleep(1600)
      } else {
        send(me, { type: 'ROLL_DICE', playerId: cur.id, roomId: room })
        await sleep(700)
      }
    } else if (a.state.phase === 'rolling') {
      turnHistory.push(`${cur.id}(rolling)`)
      await sleep(700) // autoRoll 会上报
    } else if (a.state.phase === 'turnEnd') {
      await sleep(1700)
    }
    // 每步断言双方认知一致
    if (a.state && b.state) {
      const ca = a.state.players[a.state.currentTurn]?.id
      const cb = b.state.players[b.state.currentTurn]?.id
      if (ca !== cb) throw new Error(`回合认知不一致: A看到${ca}, B看到${cb}`)
      const oa = a.state.players.map((p) => p.id).join(',')
      if (oa !== orderAtJoin) throw new Error(`玩家顺序被洗牌! ${orderAtJoin} -> ${oa}`)
    }
  }

  console.log('回合序列:', turnHistory.join(' -> '))
  const sa = JSON.stringify(a.state.players.map((p) => p.score))
  const sb = JSON.stringify(b.state.players.map((p) => p.score))
  if (sa !== sb) throw new Error(`分数不一致: ${sa} vs ${sb}`)
  console.log('多回合顺序稳定，双方始终同步')
  a.ws.close(); b.ws.close(); process.exit(0)
}
main().catch((e) => { console.error('FAIL:', e.message); process.exit(1) })
