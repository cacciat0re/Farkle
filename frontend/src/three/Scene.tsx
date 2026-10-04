import { useCallback, useMemo, useRef } from 'react'
import * as THREE from 'three'
import { Canvas, useFrame } from '@react-three/fiber'
import { Physics, RigidBody, CuboidCollider } from '@react-three/rapier'
import { Environment, ContactShadows } from '@react-three/drei'
import { Die } from './Die'
import { createWoodTexture, createLeatherTexture, createDiceFaceTexture } from './textures'
import { useGameStore, useIsMyTurn } from '../store'

const TRAY = { x: 4.2, z: 2.6 } // 托盘内沿半宽

/** 火焰摇曳的点光源 */
function Candle({ position, color = '#ff9c3f', intensity = 6 }: { position: [number, number, number]; color?: string; intensity?: number }) {
  const ref = useRef<THREE.PointLight>(null)
  const seed = useMemo(() => Math.random() * 100, [])
  useFrame(({ clock }) => {
    const t = clock.elapsedTime
    if (ref.current) {
      ref.current.intensity = intensity * (1 + Math.sin(t * 9 + seed) * 0.12 + Math.sin(t * 23 + seed * 2) * 0.06)
    }
  })
  return <pointLight ref={ref} position={position} color={color} intensity={intensity} distance={12} decay={2} castShadow shadow-mapSize={[512, 512]} />
}

function Table() {
  const wood = useMemo(() => {
    const t = createWoodTexture()
    t.repeat.set(3, 2)
    return t
  }, [])
  return (
    <group>
      {/* 桌面（整桌都是碰撞体，即使骰子翻出托盘也掉不出去） */}
      <RigidBody type="fixed" colliders="cuboid">
        <mesh receiveShadow position={[0, -0.55, 0]}>
          <boxGeometry args={[26, 1, 16]} />
          <meshStandardMaterial map={wood} roughness={0.72} metalness={0.05} />
        </mesh>
      </RigidBody>
      {/* 隐形空气墙（只有物理碰撞体，不可见） */}
      <RigidBody type="fixed" colliders={false}>
        <CuboidCollider args={[TRAY.x + 0.5, 1.2, 0.25]} position={[0, 1.2, -TRAY.z - 0.25]} />
        <CuboidCollider args={[TRAY.x + 0.5, 1.2, 0.25]} position={[0, 1.2, TRAY.z + 0.25]} />
        <CuboidCollider args={[0.25, 1.2, TRAY.z + 0.5]} position={[-TRAY.x - 0.25, 1.2, 0]} />
        <CuboidCollider args={[0.25, 1.2, TRAY.z + 0.5]} position={[TRAY.x + 0.25, 1.2, 0]} />
      </RigidBody>
      {/* 木盘底板（比空气墙略大，视觉上就是一块干净的盘子） */}
      <RigidBody type="fixed" colliders="cuboid">
        <mesh receiveShadow position={[0, -0.05, 0]}>
          <boxGeometry args={[TRAY.x * 2 + 1.6, 0.1, TRAY.z * 2 + 1.6]} />
          <meshStandardMaterial map={wood} roughness={0.6} metalness={0.03} />
        </mesh>
      </RigidBody>
    </group>
  )
}

/** 已锁定的骰子：静态陈列在托盘旁 */
function LockedDie({ value, index }: { value: number; index: number }) {
  const tex = useMemo(() => createDiceFaceTexture(value), [value])
  const x = -TRAY.x + 0.7 + index * 0.75
  return (
    <mesh castShadow position={[x, 0.28, TRAY.z + 1.4]} rotation={[-Math.PI / 2.4, 0, 0.15 * index]}>
      <boxGeometry args={[0.5, 0.5, 0.5]} />
      <meshStandardMaterial map={tex} roughness={0.32} />
    </mesh>
  )
}

function DiceRig() {
  const state = useGameStore((s) => s.state)
  const rollSeq = useGameStore((s) => s.rollSeq)
  const rollSeed = useGameStore((s) => s.rollSeed)
  const rollingCount = useGameStore((s) => s.rollingCount)
  const settledValues = useGameStore((s) => s.settledValues)
  const reportDieSettled = useGameStore((s) => s.reportDieSettled)
  const selected = useGameStore((s) => s.selected)
  const isMyTurn = useIsMyTurn()

  const lockedDice = state?.lockedDice ?? []
  const phase = state?.phase
  const scoring = phase === 'scoring' && isMyTurn

  // 始终渲染本次投掷的全部骰子（锁定只是隐藏，不卸载，保留物理状态与索引稳定）
  const count = rollingCount

  // 已锁定的骰子从托盘隐藏：所有客户端种子一致，settledValues 即每颗骰子的值，
  // 按值匹配 lockedDice 多重集（取先出现的）得出要隐藏的索引——投掷者和观战者都适用
  const hiddenSet = useMemo(() => {
    if (phase !== 'scoring' && phase !== 'turnEnd') return new Set<number>()
    const remaining = new Map<number, number>()
    for (const v of lockedDice) remaining.set(v, (remaining.get(v) ?? 0) + 1)
    const hidden = new Set<number>()
    settledValues.forEach((v, i) => {
      const c = remaining.get(v ?? -1) ?? 0
      if (c > 0) {
        hidden.add(i)
        remaining.set(v!, c - 1)
      }
    })
    return hidden
  }, [phase, lockedDice, settledValues])

  // selected 是骰子索引集合
  const selectedSet = useMemo(() => new Set(selected), [selected])

  const handleSettled = useCallback(
    (index: number, value: number) => reportDieSettled(index, value),
    [reportDieSettled],
  )

  return (
    <group>
      {Array.from({ length: count }, (_, i) => (
        <Die
          key={i}
          index={i}
          seed={rollSeed}
          rollSeq={rollSeq}
          selectable={scoring && !hiddenSet.has(i)}
          highlighted={selectedSet.has(i)}
          trayBounds={TRAY}
          hidden={hiddenSet.has(i)}
          onSettled={handleSettled}
        />
      ))}
      {lockedDice.map((v, i) => (
        <LockedDie key={`L${i}`} value={v} index={i} />
      ))}
    </group>
  )
}

/** 骰盅装饰 */
function DiceCup() {
  const leather = useMemo(() => createLeatherTexture(), [])
  return (
    <mesh castShadow position={[TRAY.x + 2.2, 0.9, -1.8]} rotation={[0.25, 0, -0.9]}>
      <cylinderGeometry args={[0.85, 0.65, 1.8, 32, 1, true]} />
      <meshStandardMaterial map={leather} roughness={0.9} side={THREE.DoubleSide} />
    </mesh>
  )
}

function CameraRig() {
  useFrame(({ camera, pointer }) => {
    // 轻微视差
    camera.position.x += (pointer.x * 1.2 - camera.position.x + 0) * 0.03
    camera.lookAt(0, 0, 0)
  })
  return null
}

export function Scene() {
  return (
    <Canvas
      shadows
      camera={{ position: [0, 7.5, 9.5], fov: 42 }}
      gl={{ antialias: true, toneMapping: THREE.ACESFilmicToneMapping, toneMappingExposure: 1.05 }}
      onCreated={({ gl, scene }) => {
        scene.background = new THREE.Color('#120a06')
        scene.fog = new THREE.Fog('#120a06', 14, 34)
        gl.shadowMap.enabled = true
        gl.shadowMap.type = THREE.PCFShadowMap
      }}
    >
      {/* 环境反射（程序化 HDR，无外部依赖） */}
      <Environment resolution={128}>
        <group>
          <mesh position={[0, 5, 0]} rotation={[Math.PI / 2, 0, 0]}>
            <planeGeometry args={[20, 20]} />
            <meshBasicMaterial color="#704a28" side={THREE.DoubleSide} />
          </mesh>
          <mesh position={[0, -4, 0]} rotation={[-Math.PI / 2, 0, 0]}>
            <planeGeometry args={[20, 20]} />
            <meshBasicMaterial color="#1a0f08" side={THREE.DoubleSide} />
          </mesh>
        </group>
      </Environment>

      <CameraRig />

      {/* 主光：酒馆吊灯暖光 */}
      <spotLight
        position={[0, 10, 2]}
        angle={0.65}
        penumbra={0.6}
        intensity={140}
        color="#ffc078"
        castShadow
        shadow-mapSize={[2048, 2048]}
        shadow-bias={-0.0004}
      />
      <ambientLight intensity={0.12} color="#4a3a55" />
      <Candle position={[-8, 1.6, -4]} />
      <Candle position={[8, 1.4, 3]} color="#ff8438" intensity={4} />
      <Candle position={[-6, 1.2, 5]} color="#ffb050" intensity={3} />

      <Physics gravity={[0, -9.81, 0]} timeStep={1 / 60}>
        <Table />
        <DiceRig />
      </Physics>

      <DiceCup />
      <ContactShadows position={[0, -0.04, 0]} opacity={0.55} scale={22} blur={2.2} far={4} />
    </Canvas>
  )
}
