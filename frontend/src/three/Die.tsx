import { useMemo, useRef, useEffect, useState } from 'react'
import * as THREE from 'three'
import { useFrame, type ThreeEvent } from '@react-three/fiber'
import { RigidBody, type RapierRigidBody } from '@react-three/rapier'
import { createDiceFaceTexture } from './textures'
import { playDiceHit } from '../audio'
import { useGameStore } from '../store'

const DIE = 0.55 // 骰子边长

// BoxGeometry 材质面顺序：[+x, -x, +y, -y, +z, -z]
// 约定：右3 左4 上1 下6 前2 后5（对面之和为 7）
const FACE_VALUES = [3, 4, 1, 6, 2, 5]

// 每个面的法向量（局部坐标），用于停稳后读取顶面点数
const FACE_NORMALS: [THREE.Vector3, number][] = [
  [new THREE.Vector3(1, 0, 0), 3],
  [new THREE.Vector3(-1, 0, 0), 4],
  [new THREE.Vector3(0, 1, 0), 1],
  [new THREE.Vector3(0, -1, 0), 6],
  [new THREE.Vector3(0, 0, 1), 2],
  [new THREE.Vector3(0, 0, -1), 5],
]

/** 从四元数读取朝上的点数 */
export function readTopValue(q: THREE.Quaternion): number {
  let best = 1
  let bestY = -Infinity
  const v = new THREE.Vector3()
  for (const [n, val] of FACE_NORMALS) {
    v.copy(n).applyQuaternion(q)
    if (v.y > bestY) {
      bestY = v.y
      best = val
    }
  }
  return best
}

// Rapier RigidBodyType 枚举：0=Dynamic, 2=KinematicPositionBased
const BODY_DYNAMIC = 0
const BODY_KINEMATIC_POSITION = 2

/** 确定性 PRNG：同一 seed 在所有客户端产生完全相同的投掷，保证动画与结果全局一致 */
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

// 让每个点数朝上的基准四元数（仅观战端物理微差时兜底校正用）
const FACE_QUATS: Record<number, THREE.Quaternion> = {
  1: new THREE.Quaternion(),
  6: new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(1, 0, 0), Math.PI),
  2: new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(1, 0, 0), -Math.PI / 2),
  5: new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(1, 0, 0), Math.PI / 2),
  3: new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(0, 0, 1), Math.PI / 2),
  4: new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(0, 0, 1), -Math.PI / 2),
}

interface DieProps {
  index: number
  seed: number // 本次投掷的种子（服务端下发，全桌一致）
  rollSeq: number
  selectable: boolean
  highlighted: boolean
  trayBounds: { x: number; z: number }
  /** 非空时：观战端物理结果与服务端不一致，需平滑校正到该点数 */
  correctTo?: number | null
  /** 为 true 时隐藏并从物理世界移除（已锁定的骰子） */
  hidden?: boolean
  /** 物理停稳后回调实际顶面点数 */
  onSettled?: (index: number, value: number) => void
}

type DiePhase = 'idle' | 'rolling' | 'correcting' | 'settled'

export function Die({ index, seed, rollSeq, selectable, highlighted, trayBounds, correctTo, hidden, onSettled }: DieProps) {
  const body = useRef<RapierRigidBody>(null)
  const phase = useRef<DiePhase>('idle')
  const settleTimer = useRef(0)
  const correctT = useRef(0)
  const startQuat = useRef(new THREE.Quaternion())
  const targetQuat = useRef(new THREE.Quaternion())
  const settledValue = useRef<number | null>(null)
  const correctedFor = useRef<number | null>(null)
  const toggleSelect = useGameStore((s) => s.toggleSelect)
  const [hovered, setHovered] = useState(false)

  const materials = useMemo(
    () =>
      FACE_VALUES.map(
        (v) =>
          new THREE.MeshStandardMaterial({
            map: createDiceFaceTexture(v),
            roughness: 0.32,
            metalness: 0.02,
            envMapIntensity: 0.7,
          }),
      ),
    [],
  )

  // 投掷触发：rollSeq 变化时用种子 PRNG 给刚体冲量——所有客户端结果完全一致
  useEffect(() => {
    if (rollSeq === 0) return
    const api = body.current
    if (!api) return

    phase.current = 'rolling'
    settleTimer.current = 0
    settledValue.current = null
    correctedFor.current = null

    const rng = mulberry32(seed + index * 7919)
    const rnd = (a: number, b: number) => a + rng() * (b - a)

    const x = rnd(-trayBounds.x * 0.6, trayBounds.x * 0.6)
    const z = rnd(-trayBounds.z * 0.6, trayBounds.z * 0.6)
    api.setBodyType(BODY_DYNAMIC, true)
    api.setTranslation({ x, y: 4.5 + index * 0.25, z }, true)
    api.setRotation(
      new THREE.Quaternion().setFromEuler(
        new THREE.Euler(rnd(0, Math.PI * 2), rnd(0, Math.PI * 2), rnd(0, Math.PI * 2)),
      ),
      true,
    )
    api.setLinvel({ x: rnd(-2.5, 2.5), y: rnd(-2, -0.5), z: rnd(-2.5, 2.5) }, true)
    api.setAngvel({ x: rnd(-14, 14), y: rnd(-14, 14), z: rnd(-14, 14) }, true)
    api.wakeUp()
  }, [rollSeq, seed, index, trayBounds])

  useFrame((_, dt) => {
    const api = body.current
    if (!api) return

    if (phase.current === 'rolling') {
      // 兜底：意外穿模坠落时拉回托盘上方（非确定性场景，直接用 Math.random）
      if (api.translation().y < -1) {
        api.setTranslation(
          {
            x: (Math.random() * 2 - 1) * trayBounds.x * 0.6,
            y: 3,
            z: (Math.random() * 2 - 1) * trayBounds.z * 0.6,
          },
          true,
        )
        api.setLinvel({ x: 0, y: -1, z: 0 }, true)
      }
      const lv = api.linvel()
      const av = api.angvel()
      const speed = Math.hypot(lv.x, lv.y, lv.z) + Math.hypot(av.x, av.y, av.z)
      if (speed < 0.35) {
        settleTimer.current += dt
      } else {
        settleTimer.current = 0
      }
      // 静止足够久 → 读取物理结果（动画即真理，投掷者无需任何校正）
      if (settleTimer.current > 0.3) {
        phase.current = 'settled'
        const rq = api.rotation()
        const value = readTopValue(new THREE.Quaternion(rq.x, rq.y, rq.z, rq.w))
        settledValue.current = value
        onSettled?.(index, value)
      }
    } else if (phase.current === 'settled' && correctTo != null && correctTo !== settledValue.current && correctedFor.current !== correctTo) {
      // 仅观战端可能出现：本地物理与服务端结果微差（浮点/帧率），平滑校正一次
      correctedFor.current = correctTo
      phase.current = 'correcting'
      correctT.current = 0
      const rq = api.rotation()
      startQuat.current.set(rq.x, rq.y, rq.z, rq.w)
      targetQuat.current
        .setFromAxisAngle(new THREE.Vector3(0, 1, 0), (seed % 628) / 100)
        .multiply(FACE_QUATS[correctTo])
      api.setLinvel({ x: 0, y: 0, z: 0 }, true)
      api.setAngvel({ x: 0, y: 0, z: 0 }, true)
      api.setBodyType(BODY_KINEMATIC_POSITION, true)
    } else if (phase.current === 'correcting') {
      correctT.current += dt * 4
      const t = Math.min(1, correctT.current)
      const eased = 1 - Math.pow(1 - t, 3)
      const q = new THREE.Quaternion().slerpQuaternions(
        startQuat.current,
        targetQuat.current,
        eased,
      )
      api.setNextKinematicRotation(q)
      if (t >= 1) {
        phase.current = 'settled'
        settledValue.current = correctTo ?? settledValue.current
        if (settledValue.current != null) onSettled?.(index, settledValue.current)
        api.sleep()
      }
    }
  })

  // 已锁定的骰子：刚体休眠隐藏，不再参与碰撞，也不再渲染
  useEffect(() => {
    const api = body.current
    if (!api) return
    if (hidden) {
      api.sleep()
      api.setEnabled(false)
    } else {
      api.setEnabled(true)
      api.wakeUp()
    }
  }, [hidden])

  const onClick = (e: ThreeEvent<MouseEvent>) => {
    if (!selectable) return
    e.stopPropagation()
    toggleSelect(index)
  }

  const emissive = highlighted ? '#7a5cff' : hovered && selectable ? '#3d5a80' : '#000000'
  const emissiveIntensity = highlighted ? 0.55 : hovered && selectable ? 0.25 : 0

  return (
    <RigidBody
      ref={body}
      colliders="cuboid"
      restitution={0.38}
      friction={0.85}
      density={1.4}
      ccd
      onCollisionEnter={() => {
        const api = body.current
        if (!api) return
        const lv = api.linvel()
        const speed = Math.hypot(lv.x, lv.y, lv.z)
        if (speed > 1.2) playDiceHit(Math.min(1, speed / 9))
      }}
      userData={{ dieIndex: index }}
    >
      <mesh
        visible={!hidden}
        castShadow
        receiveShadow
        onClick={onClick}
        onPointerOver={(e) => {
          if (selectable) {
            e.stopPropagation()
            setHovered(true)
          }
        }}
        onPointerOut={() => setHovered(false)}
      >
        <boxGeometry args={[DIE, DIE, DIE]} />
        {materials.map((m, i) => (
          <primitive key={i} object={m} attach={`material-${i}`} />
        ))}
        {/* 选中高亮覆盖层 */}
        <mesh scale={1.02}>
          <boxGeometry args={[DIE, DIE, DIE]} />
          <meshStandardMaterial
            color="#000000"
            emissive={emissive}
            emissiveIntensity={emissiveIntensity}
            transparent
            opacity={highlighted ? 0.35 : 0.2}
            depthWrite={false}
          />
        </mesh>
      </mesh>
    </RigidBody>
  )
}
