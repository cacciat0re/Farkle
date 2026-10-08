/**
 * 界面音效用 WebAudio 实时合成，不依赖音频文件。
 * 浏览器要求音频必须由用户手势解锁，所以第一次 pointerdown 时会创建/恢复 AudioContext。
 */

let ctx: AudioContext | null = null
let master: GainNode | null = null

function ensureAudio(): AudioContext | null {
  if (!ctx) {
    const Ctor =
      window.AudioContext ??
      (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext
    if (!Ctor) return null
    try {
      ctx = new Ctor()
    } catch {
      return null
    }
    master = ctx.createGain()
    master.gain.value = 0.45
    // 多颗骰子/连续点击同时发声时压一下峰值，避免爆音
    const comp = ctx.createDynamicsCompressor()
    master.connect(comp)
    comp.connect(ctx.destination)
  }
  if (ctx.state === 'suspended') void ctx.resume()
  return ctx
}

/** 在用户手势里调用一次，解锁音频 */
export function unlockAudio() {
  ensureAudio()
}

/** 按钮点击：短促的木质"嗒" */
export function playClick() {
  const ac = ensureAudio()
  if (!ac || !master) return
  const t = ac.currentTime

  const osc = ac.createOscillator()
  osc.type = 'triangle'
  osc.frequency.setValueAtTime(1150, t)
  osc.frequency.exponentialRampToValueAtTime(560, t + 0.05)

  const gain = ac.createGain()
  gain.gain.setValueAtTime(0.0001, t)
  gain.gain.exponentialRampToValueAtTime(0.12, t + 0.006)
  gain.gain.exponentialRampToValueAtTime(0.0001, t + 0.07)

  osc.connect(gain).connect(master)
  osc.start(t)
  osc.stop(t + 0.08)
}

/**
 * 骰子选择：选中是上行短音，取消是下行短音。
 * 两者音色一致、只有音高方向不同，玩家一耳朵就能分辨。
 */
export function playSelect(selected: boolean) {
  const ac = ensureAudio()
  if (!ac || !master) return
  const t = ac.currentTime

  const osc = ac.createOscillator()
  osc.type = 'sine'
  const from = selected ? 620 : 520
  const to = selected ? 930 : 360
  osc.frequency.setValueAtTime(from, t)
  osc.frequency.exponentialRampToValueAtTime(to, t + 0.09)

  const gain = ac.createGain()
  gain.gain.setValueAtTime(0.0001, t)
  gain.gain.exponentialRampToValueAtTime(0.1, t + 0.008)
  gain.gain.exponentialRampToValueAtTime(0.0001, t + 0.11)

  osc.connect(gain).connect(master)
  osc.start(t)
  osc.stop(t + 0.12)
}
