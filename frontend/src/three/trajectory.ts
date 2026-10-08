import * as THREE from 'three'
import * as RAPIERImport from '@dimforge/rapier3d-compat'
import { DIE_SIZE } from '../game/layout'

/**
 * 离线预演用的 rapier wasm：懒加载并只初始化一次。
 * 场景里不再需要物理世界（骰子完全由关键帧驱动），所以这里自己负责 init。
 */
let rapierReady: Promise<typeof RAPIERImport> | null = null

function loadRapier(): Promise<typeof RAPIERImport> {
  if (!rapierReady) {
    // compat 版需要先 init 才能创建 World / ColliderDesc
    rapierReady = (RAPIERImport as unknown as { init: () => Promise<void> })
      .init()
      .then(() => RAPIERImport)
  }
  return rapierReady
}

// 与场景保持一致的常量
const DIE = DIE_SIZE
const DIE_HALF = DIE / 2
export const PHYSICS_DT = 1 / 60

// 采样与截止条件
const STEPS_PER_FRAME = 3 // ≈20Hz 关键帧
const MAX_STEPS = 900 // 15s 上限，防止跑飞
const SETTLE_SPEED = 0.35
const SETTLE_STEPS = 18

/** 每颗骰子每帧 7 个数：x,y,z,qx,qy,qz,qw */
export const NUMBERS_PER_DIE = 7

/** 一帧的步长随本次投掷的骰子数变化（锁骰后只剩 N 颗） */
export function frameStride(diceCount: number): number {
  return Math.max(1, diceCount) * NUMBERS_PER_DIE
}

export interface PrecomputedRoll {
  values: number[]
  frames: number[]
  stepsPerFrame: number
  diceCount: number
}

/** 与 Die 中一致的确定性 PRNG，保证同一 seed 每次预演结果相同 */
function mulberry32(seed: number) {
  let a = seed >>> 0
  return () => {
    a |= 0
    a = (a + 0x6d2b79f5) | 0
    let t = Math.imul(a ^ (a >>> 15), 1 | a)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

// 与 Die 相同的面法线约定：右3 左4 上1 下6 前2 后5
const FACE_NORMALS: [THREE.Vector3, number][] = [
  [new THREE.Vector3(1, 0, 0), 3],
  [new THREE.Vector3(-1, 0, 0), 4],
  [new THREE.Vector3(0, 1, 0), 1],
  [new THREE.Vector3(0, -1, 0), 6],
  [new THREE.Vector3(0, 0, 1), 2],
  [new THREE.Vector3(0, 0, -1), 5],
]

const _v = new THREE.Vector3()
function readTopValue(q: THREE.Quaternion): number {
  let best = 1
  let bestY = -Infinity
  for (const [n, val] of FACE_NORMALS) {
    _v.copy(n).applyQuaternion(q)
    if (_v.y > bestY) {
      bestY = _v.y
      best = val
    }
  }
  return best
}

/**
 * 关键帧只保留 4 位小数：位置精确到 0.1mm、四元数到 0.0001，
 * 肉眼完全看不出差别，却能把 JSON 体积压掉一半以上（这是要过 WebSocket 的消息）。
 */
function round4(v: number): number {
  return Math.round(v * 1e4) / 1e4
}

/**
 * 在独立世界里一次性预演整段投掷，返回关键帧轨迹与最终点数。
 * 只在一台机器（投掷者）上执行，因此不需要跨设备确定性。
 */
export async function precomputeRoll(
  seed: number,
  count: number,
  tray: { x: number; z: number },
): Promise<PrecomputedRoll> {
  const RAPIER: any = await loadRapier()
  const world = new RAPIER.World({ x: 0, y: -9.81, z: 0 })

  // 地面（顶面 y = 0）与四面空气墙，和场景里的托盘边界一致
  world.createCollider(
    RAPIER.ColliderDesc.cuboid(tray.x + 1, 0.05, tray.z + 1).setTranslation(0, -0.05, 0),
  )
  const wallH = 3
  world.createCollider(RAPIER.ColliderDesc.cuboid(tray.x + 0.5, wallH, 0.25).setTranslation(0, wallH, -tray.z - 0.25))
  world.createCollider(RAPIER.ColliderDesc.cuboid(tray.x + 0.5, wallH, 0.25).setTranslation(0, wallH, tray.z + 0.25))
  world.createCollider(RAPIER.ColliderDesc.cuboid(0.25, wallH, tray.z + 0.5).setTranslation(-tray.x - 0.25, wallH, 0))
  world.createCollider(RAPIER.ColliderDesc.cuboid(0.25, wallH, tray.z + 0.5).setTranslation(tray.x + 0.25, wallH, 0))

  const bodies: any[] = []
  for (let i = 0; i < count; i++) {
    const rng = mulberry32(seed + i * 7919)
    const rnd = (a: number, b: number) => a + rng() * (b - a)
    const quat = new THREE.Quaternion().setFromEuler(
      new THREE.Euler(rnd(0, Math.PI * 2), rnd(0, Math.PI * 2), rnd(0, Math.PI * 2)),
    )
    const body = world.createRigidBody(
      RAPIER.RigidBodyDesc.dynamic()
        .setTranslation(rnd(-tray.x * 0.6, tray.x * 0.6), 4.5 + i * 0.25, rnd(-tray.z * 0.6, tray.z * 0.6))
        .setRotation({ x: quat.x, y: quat.y, z: quat.z, w: quat.w })
        .setLinvel(rnd(-2.5, 2.5), rnd(-2, -0.5), rnd(-2.5, 2.5))
        .setAngvel({ x: rnd(-14, 14), y: rnd(-14, 14), z: rnd(-14, 14) })
        .setCcdEnabled(true),
    )
    world.createCollider(
      RAPIER.ColliderDesc.cuboid(DIE_HALF, DIE_HALF, DIE_HALF)
        .setRestitution(0.38)
        .setFriction(0.85)
        .setDensity(1.4),
      body,
    )
    bodies.push(body)
  }

  const frames: number[] = []
  const settle = new Array<number>(count).fill(0)
  const done = new Array<boolean>(count).fill(false)

  const pushFrame = () => {
    for (const body of bodies) {
      const t = body.translation()
      const q = body.rotation()
      frames.push(round4(t.x), round4(t.y), round4(t.z), round4(q.x), round4(q.y), round4(q.z), round4(q.w))
    }
  }
  pushFrame() // 第 0 帧 = 初始状态

  for (let step = 1; step <= MAX_STEPS; step++) {
    world.step()
    for (let i = 0; i < count; i++) {
      if (done[i]) continue
      const lv = bodies[i].linvel()
      const av = bodies[i].angvel()
      const speed = Math.hypot(lv.x, lv.y, lv.z) + Math.hypot(av.x, av.y, av.z)
      settle[i] = speed < SETTLE_SPEED ? settle[i] + 1 : 0
      if (settle[i] >= SETTLE_STEPS) {
        done[i] = true
        bodies[i].setBodyType(RAPIER.RigidBodyType.Fixed, true)
      }
    }
    if (step % STEPS_PER_FRAME === 0) pushFrame()
    if (done.every(Boolean)) break
  }
  pushFrame() // 末帧 = 权威终态

  const values = bodies.map((body) => {
    const q = body.rotation()
    return readTopValue(new THREE.Quaternion(q.x, q.y, q.z, q.w))
  })

  world.free()
  return { values, frames, stepsPerFrame: STEPS_PER_FRAME, diceCount: count }
}

/** 天顶朝上的标准姿态：把点数为 value 的那一面转到 +y */
export function faceUpQuaternion(value: number): THREE.Quaternion {
  const up = new THREE.Vector3(0, 1, 0)
  const entry = FACE_NORMALS.find(([, v]) => v === value) ?? FACE_NORMALS[2]
  return new THREE.Quaternion().setFromUnitVectors(entry[0], up)
}

/**
 * 物理预演不可用时的兜底动画（纯数学，不依赖 wasm）：
 * 保证骰子仍然会掉落、停稳，且朝上的点数与结算点数一致，
 * 不会出现"点数已经算完但画面还是初始摆位"的错位。
 */
export function syntheticRoll(seed: number, count: number, tray: { x: number; z: number }): PrecomputedRoll {
  const frames = 24
  const values: number[] = []
  const plans: {
    startX: number
    startY: number
    startZ: number
    endX: number
    endY: number
    endZ: number
    spin: THREE.Quaternion
    final: THREE.Quaternion
  }[] = []

  for (let i = 0; i < count; i++) {
    const rng = mulberry32(seed + i * 7919)
    const rnd = (a: number, b: number) => a + rng() * (b - a)
    const value = 1 + Math.floor(rng() * 6)
    values.push(value)
    plans.push({
      startX: rnd(-tray.x * 0.6, tray.x * 0.6),
      startY: 4.2 + i * 0.25,
      startZ: rnd(-tray.z * 0.6, tray.z * 0.6),
      endX: ((i % 3) - 1) * 1.1 + rnd(-0.2, 0.2),
      endY: DIE_HALF + 0.001,
      endZ: (Math.floor(i / 3) - 0.5) * 1.2 + rnd(-0.2, 0.2),
      spin: new THREE.Quaternion().setFromEuler(
        new THREE.Euler(rnd(0, Math.PI * 2), rnd(0, Math.PI * 2), rnd(0, Math.PI * 2)),
      ),
      final: faceUpQuaternion(value),
    })
  }

  // 帧优先（每帧依次写入所有骰子），与 precomputeRoll 的布局保持一致
  const out: number[] = []
  for (let f = 0; f < frames; f++) {
    const t = f / (frames - 1)
    for (const p of plans) {
      const x = p.startX + (p.endX - p.startX) * t
      const z = p.startZ + (p.endZ - p.startZ) * t
      // 抛物线落地：先升后落，t=1 时正好贴住盘面
      const y = p.startY + (p.endY - p.startY) * t * t + Math.sin(Math.PI * t) * 0.5
      const q = p.spin.clone().slerp(p.final, Math.min(1, t * 1.15))
      out.push(round4(x), round4(y), round4(z), round4(q.x), round4(q.y), round4(q.z), round4(q.w))
    }
  }

  return { values, frames: out, stepsPerFrame: STEPS_PER_FRAME, diceCount: count }
}
