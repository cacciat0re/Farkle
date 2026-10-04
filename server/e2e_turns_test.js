// 多回合顺序稳定性测试：node e2e_turns_test.js
const WebSocket = require('ws')
const room = 'turns-' + Date.now()
const url = `ws://localhost:8080/ws?room=${room}`
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

function client(name) {
  const ws = new WebSocket(url)
  const c = { ws, name, state: null, lastRoll: null }
  ws.on('message', (d) => {
    const m = JSON.parse(d)
    if (m.type === 'GAME_STATE') c.state = m.state
    if (m.type === 'DICE_ROLLED') { c.state = m.state; c.lastRoll = m.roll }
  })
  return new Promise((res) => ws.on('open', () => res(c)))
}

async function main() {
  const a = await client('Alice')
  const b = await client('Bob')
  a.ws.send(JSON.stringify({ type: 'JOIN_ROOM', playerId: 'a', name: 'Alice', roomId: room }))
  b.ws.send(JSON.stringify({ type: 'JOIN_ROOM', playerId: 'b', name: 'Bob', roomId: room }))
  await sleep(300)

  // 记录加入顺序
  const orderAtJoin = a.state.players.map((p) => p.id).join(',')
  console.log('加入顺序:', orderAtJoin)

  a.ws.send(JSON.stringify({ type: 'START_GAME', playerId: 'a', roomId: room }))
  await sleep(3200)

  const turnHistory = []
  let lastTurnIdx = -1

  // 连续打 6 个回合：每个回合锁定得分骰并入库或继续掷
  for (let round = 0; round < 6; round++) {
    const cur = a.state.players[a.state.currentTurn]
    turnHistory.push(`${cur.id}(gen${a.state.rollCount})`)

    // 双方对当前回合玩家的认知必须一致
    const curB = b.state.players[b.state.currentTurn]
    if (cur.id !== curB.id) throw new Error(`回合认知不一致: A看到${cur.id}, B看到${curB.id}`)

    // 玩家顺序在所有广播后必须保持不变
    const orderNow = a.state.players.map((p) => p.id).join(',')
    if (orderNow !== orderAtJoin) throw new Error(`玩家顺序被洗牌! ${orderAtJoin} -> ${orderNow}`)

    const me = cur.id === 'a' ? a : b
    const mid = cur.id
    const keep = me.lastRoll.filter((v) => v === 1 || v === 5)
    if (keep.length > 0) {
      me.ws.send(JSON.stringify({ type: 'KEEP_DICE', playerId: mid, roomId: room, keep }))
      await sleep(300)
    }
    if (me.state.turnScore >= 300) {
      me.ws.send(JSON.stringify({ type: 'BANK_SCORE', playerId: mid, roomId: room }))
      await sleep(1800) // 入库动画 + 轮转
    } else {
      me.ws.send(JSON.stringify({ type: 'ROLL_DICE', playerId: mid, roomId: room }))
      await sleep(3200)
    }
  }

  console.log('回合序列:', turnHistory.join(' -> '))

  // 双方分数一致
  const sa = JSON.stringify(a.state.players.map((p) => p.score))
  const sb = JSON.stringify(b.state.players.map((p) => p.score))
  if (sa !== sb) throw new Error(`分数不一致: ${sa} vs ${sb}`)

  console.log('✅ 多回合顺序稳定，双方始终同步')
  a.ws.close(); b.ws.close(); process.exit(0)
}
main().catch((e) => { console.error('❌ FAIL:', e.message); process.exit(1) })
