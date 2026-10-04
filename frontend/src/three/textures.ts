import * as THREE from 'three'

/** 用 Canvas 程序化生成木纹贴图（含法线感的明暗条纹） */
export function createWoodTexture(size = 1024): THREE.CanvasTexture {
  const c = document.createElement('canvas')
  c.width = c.height = size
  const ctx = c.getContext('2d')!

  // 底色：深胡桃木
  ctx.fillStyle = '#4a2f1d'
  ctx.fillRect(0, 0, size, size)

  // 木板条
  const planks = 6
  const plankH = size / planks
  for (let p = 0; p < planks; p++) {
    const y = p * plankH
    const tone = 0.85 + Math.random() * 0.3
    ctx.fillStyle = `rgb(${74 * tone | 0},${47 * tone | 0},${29 * tone | 0})`
    ctx.fillRect(0, y, size, plankH)
    // 木纹曲线
    for (let i = 0; i < 90; i++) {
      const gy = y + Math.random() * plankH
      const alpha = 0.04 + Math.random() * 0.10
      ctx.strokeStyle = Math.random() > 0.5
        ? `rgba(30,16,8,${alpha})`
        : `rgba(140,95,55,${alpha})`
      ctx.lineWidth = 0.6 + Math.random() * 2.2
      ctx.beginPath()
      ctx.moveTo(0, gy)
      for (let x = 0; x <= size; x += 32) {
        ctx.lineTo(x, gy + Math.sin(x * 0.01 + p * 3 + i) * 4 * Math.random())
      }
      ctx.stroke()
    }
    // 板间缝隙
    ctx.fillStyle = 'rgba(0,0,0,0.55)'
    ctx.fillRect(0, y, size, 3)
  }

  // 噪点
  const img = ctx.getImageData(0, 0, size, size)
  for (let i = 0; i < img.data.length; i += 4) {
    const n = (Math.random() - 0.5) * 14
    img.data[i] += n
    img.data[i + 1] += n
    img.data[i + 2] += n
  }
  ctx.putImageData(img, 0, 0)

  const tex = new THREE.CanvasTexture(c)
  tex.wrapS = tex.wrapT = THREE.RepeatWrapping
  tex.anisotropy = 8
  tex.colorSpace = THREE.SRGBColorSpace
  return tex
}

/** 骰子面贴图：骨白底 + 深色凹陷点数 */
export function createDiceFaceTexture(value: number, size = 256): THREE.CanvasTexture {
  const c = document.createElement('canvas')
  c.width = c.height = size
  const ctx = c.getContext('2d')!

  // 骨质底色，带轻微径向渐变
  const g = ctx.createRadialGradient(size / 2, size / 2, size * 0.1, size / 2, size / 2, size * 0.75)
  g.addColorStop(0, '#f3ead6')
  g.addColorStop(1, '#d9cbaa')
  ctx.fillStyle = g
  ctx.fillRect(0, 0, size, size)

  // 细微噪点（骨材颗粒感）
  const img = ctx.getImageData(0, 0, size, size)
  for (let i = 0; i < img.data.length; i += 4) {
    const n = (Math.random() - 0.5) * 10
    img.data[i] += n
    img.data[i + 1] += n
    img.data[i + 2] += n * 0.8
  }
  ctx.putImageData(img, 0, 0)

  const pip = (fx: number, fy: number) => {
    const x = fx * size
    const y = fy * size
    const r = size * 0.085
    // 凹陷阴影
    ctx.fillStyle = 'rgba(0,0,0,0.35)'
    ctx.beginPath()
    ctx.arc(x, y + r * 0.25, r, 0, Math.PI * 2)
    ctx.fill()
    // 点体
    const pg = ctx.createRadialGradient(x - r * 0.3, y - r * 0.3, r * 0.1, x, y, r)
    pg.addColorStop(0, '#3a2a1a')
    pg.addColorStop(1, '#120c06')
    ctx.fillStyle = pg
    ctx.beginPath()
    ctx.arc(x, y, r, 0, Math.PI * 2)
    ctx.fill()
  }

  const P: Record<number, [number, number][]> = {
    1: [[0.5, 0.5]],
    2: [[0.27, 0.27], [0.73, 0.73]],
    3: [[0.25, 0.25], [0.5, 0.5], [0.75, 0.75]],
    4: [[0.28, 0.28], [0.72, 0.28], [0.28, 0.72], [0.72, 0.72]],
    5: [[0.26, 0.26], [0.74, 0.26], [0.5, 0.5], [0.26, 0.74], [0.74, 0.74]],
    6: [[0.28, 0.24], [0.72, 0.24], [0.28, 0.5], [0.72, 0.5], [0.28, 0.76], [0.72, 0.76]],
  }
  for (const [fx, fy] of P[value]) pip(fx, fy)

  const tex = new THREE.CanvasTexture(c)
  tex.anisotropy = 8
  tex.colorSpace = THREE.SRGBColorSpace
  return tex
}

/** 绿毡（骰盘台面）贴图——与木桌明显区分 */
export function createFeltTexture(size = 512): THREE.CanvasTexture {
  const c = document.createElement('canvas')
  c.width = c.height = size
  const ctx = c.getContext('2d')!
  ctx.fillStyle = '#2e5d3a'
  ctx.fillRect(0, 0, size, size)
  const img = ctx.getImageData(0, 0, size, size)
  for (let i = 0; i < img.data.length; i += 4) {
    const n = (Math.random() - 0.5) * 18
    img.data[i] += n * 0.6
    img.data[i + 1] += n
    img.data[i + 2] += n * 0.6
  }
  ctx.putImageData(img, 0, 0)
  const tex = new THREE.CanvasTexture(c)
  tex.wrapS = tex.wrapT = THREE.RepeatWrapping
  tex.colorSpace = THREE.SRGBColorSpace
  return tex
}

/** 皮革（骰盅/托盘边框）贴图 */
export function createLeatherTexture(size = 512): THREE.CanvasTexture {
  const c = document.createElement('canvas')
  c.width = c.height = size
  const ctx = c.getContext('2d')!
  ctx.fillStyle = '#3d2314'
  ctx.fillRect(0, 0, size, size)
  const img = ctx.getImageData(0, 0, size, size)
  for (let i = 0; i < img.data.length; i += 4) {
    const n = (Math.random() - 0.5) * 26
    img.data[i] += n
    img.data[i + 1] += n * 0.7
    img.data[i + 2] += n * 0.5
  }
  ctx.putImageData(img, 0, 0)
  const tex = new THREE.CanvasTexture(c)
  tex.wrapS = tex.wrapT = THREE.RepeatWrapping
  tex.colorSpace = THREE.SRGBColorSpace
  return tex
}
