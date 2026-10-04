// 用 WebAudio 实时合成骰子碰撞声与酒馆氛围，无需音频文件
let ctx: AudioContext | null = null

function ensureCtx(): AudioContext | null {
  if (!ctx) {
    try {
      ctx = new AudioContext()
    } catch {
      return null
    }
  }
  if (ctx.state === 'suspended') void ctx.resume()
  return ctx
}

/** 根据撞击强度播放骰子碰撞声（骨/木混合噪声） */
export function playDiceHit(intensity: number) {
  const ac = ensureCtx()
  if (!ac) return
  const vol = Math.min(1, Math.max(0.05, intensity))
  const dur = 0.06 + vol * 0.05

  const bufferSize = ac.sampleRate * dur
  const buffer = ac.createBuffer(1, bufferSize, ac.sampleRate)
  const data = buffer.getChannelData(0)
  for (let i = 0; i < bufferSize; i++) {
    data[i] = (Math.random() * 2 - 1) * Math.pow(1 - i / bufferSize, 2.2)
  }
  const src = ac.createBufferSource()
  src.buffer = buffer

  const filter = ac.createBiquadFilter()
  filter.type = 'bandpass'
  filter.frequency.value = 900 + Math.random() * 1400
  filter.Q.value = 1.2

  const gain = ac.createGain()
  gain.gain.value = vol * 0.5

  src.connect(filter).connect(gain).connect(ac.destination)
  src.start()

  // 低频木质“咚”
  const osc = ac.createOscillator()
  osc.type = 'sine'
  osc.frequency.value = 140 + Math.random() * 60
  const og = ac.createGain()
  og.gain.setValueAtTime(vol * 0.25, ac.currentTime)
  og.gain.exponentialRampToValueAtTime(0.001, ac.currentTime + 0.09)
  osc.connect(og).connect(ac.destination)
  osc.start()
  osc.stop(ac.currentTime + 0.1)
}

let ambientNodes: AudioNode[] | null = null

/** 酒馆环境背景音（循环篝火噼啪 + 低吟氛围） */
export function startAmbient() {
  const ac = ensureCtx()
  if (!ac || ambientNodes) return

  // 暖色底噪
  const len = ac.sampleRate * 2
  const buffer = ac.createBuffer(1, len, ac.sampleRate)
  const data = buffer.getChannelData(0)
  let last = 0
  for (let i = 0; i < len; i++) {
    // 布朗噪声，更像酒馆低鸣
    last = (last + (Math.random() * 2 - 1) * 0.02) * 0.998
    data[i] = last * 8
  }
  const src = ac.createBufferSource()
  src.buffer = buffer
  src.loop = true
  const lp = ac.createBiquadFilter()
  lp.type = 'lowpass'
  lp.frequency.value = 240
  const gain = ac.createGain()
  gain.gain.value = 0.05
  src.connect(lp).connect(gain).connect(ac.destination)
  src.start()
  ambientNodes = [src, lp, gain]
}
