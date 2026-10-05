import { useEffect, useMemo, useRef } from 'react'
import * as THREE from 'three'
import { Canvas, useFrame } from '@react-three/fiber'
import { Physics, RigidBody, CuboidCollider } from '@react-three/rapier'
import { Environment, ContactShadows } from '@react-three/drei'
import { Die } from './Die'
import { createWoodTexture, createDiceFaceTexture, createFeltTexture } from './textures'
import { selectableIndices } from '../game/scoring'
import { hiddenIndices } from '../game/selection'
import { useGameStore, useIsMyTurn } from '../store'
import { PHYSICS_STEP } from './physics'
import { NUMBERS_PER_DIE, setRapierModule } from './trajectory'
import { useRapier } from '@react-three/rapier'

const TRAY = { x: 4.2, z: 2.6 } // 托盘内沿半宽

/** 火焰摇曳的点光源 */
function Candle({ position, color = '#ff9c3f', intensity = 6 }: { position: [number, number, number]; color?: string; intensity?: number }) {
  const ref = useRef<THREE.PointLight>(null)
  // 固定相位：不用 Math.random，保证两端看到同一套烛光闪烁
  const seed = 17.3
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
      {/* 隐形空气墙：高 6 米，骰子绝无可能翻出盘子 */}
      <RigidBody type="fixed" colliders={false}>
        <CuboidCollider args={[TRAY.x + 0.5, 3, 0.25]} position={[0, 3, -TRAY.z - 0.25]} />
        <CuboidCollider args={[TRAY.x + 0.5, 3, 0.25]} position={[0, 3, TRAY.z + 0.25]} />
        <CuboidCollider args={[0.25, 3, TRAY.z + 0.5]} position={[-TRAY.x - 0.25, 3, 0]} />
        <CuboidCollider args={[0.25, 3, TRAY.z + 0.5]} position={[TRAY.x + 0.25, 3, 0]} />
      </RigidBody>
      {/* 绿毡骰盘（与木桌明显区分） */}
      <RigidBody type="fixed" colliders="cuboid">
        <mesh receiveShadow position={[0, -0.05, 0]}>
          <boxGeometry args={[TRAY.x * 2 + 1.6, 0.1, TRAY.z * 2 + 1.6]} />
          <meshStandardMaterial map={createFeltTexture()} roughness={0.95} metalness={0} />
        </mesh>
      </RigidBody>
    </group>
  )
}

function TableCandle({ position }: { position: [number, number, number] }) {
  return (
    <group position={position}>
      <mesh castShadow receiveShadow position={[0, 0.018, 0]}>
        <cylinderGeometry args={[0.36, 0.38, 0.036, 24]} />
        <meshStandardMaterial color="#6a4427" roughness={0.92} />
      </mesh>
      <mesh castShadow position={[0, 0.28, 0]}>
        <cylinderGeometry args={[0.14, 0.17, 0.48, 16]} />
        <meshStandardMaterial color="#d8c8a2" roughness={0.82} />
      </mesh>
      <mesh position={[0, 0.61, 0]}>
        <coneGeometry args={[0.065, 0.18, 12]} />
        <meshBasicMaterial color="#ffbd57" />
      </mesh>
    </group>
  )
}

function TableDecor() {
  return (
    <group>
      {/* 放在桌面右上角，避开左上角的聊天框 */}
      <TableCandle position={[6.6, 0, -4]} />
    </group>
  )
}

/** 已锁定的骰子：静态陈列在托盘旁 */
function LockedDie({ value, index, opposite }: { value: number; index: number; opposite: boolean }) {
  const tex = useMemo(() => createDiceFaceTexture(value), [value])
  const x = -TRAY.x + 0.7 + index * 0.68
  const z = opposite ? -TRAY.z - 1.4 : TRAY.z + 1.4
  const tilt = opposite ? Math.PI / 2.4 : -Math.PI / 2.4
  return (
    <mesh castShadow position={[x, 0.23, z]} rotation={[tilt, 0, 0.15 * index]}>
      <boxGeometry args={[0.44, 0.44, 0.44]} />
      <meshStandardMaterial map={tex} roughness={0.32} />
    </mesh>
  )
}

function DiceRig() {
  // @react-three/rapier 已完成 wasm 初始化，把它交给离线预演复用，避免二次 init 卡住
  const { rapier } = useRapier()
  useEffect(() => {
    setRapierModule(rapier)
  }, [rapier])

  const state = useGameStore((s) => s.state)
  const rollingCount = useGameStore((s) => s.rollingCount)
  const trajectory = useGameStore((s) => s.trajectory)
  const trajectoryDice = useGameStore((s) => s.trajectoryDice)
  const trajectoryLive = useGameStore((s) => s.trajectoryLive)
  const stepsPerFrame = useGameStore((s) => s.stepsPerFrame)
  const settledValues = useGameStore((s) => s.settledValues)
  const selected = useGameStore((s) => s.selected)
  const isMyTurn = useIsMyTurn()

  const lockedDice = state?.lockedDice
  const remainingDice = state?.diceValues
  const phase = state?.phase
  const scoring = phase === 'scoring' && isMyTurn
  // 本轮关键帧轨迹：投掷者离线预演后广播，所有人（含投掷者）回放同一份
  const playing =
    !!trajectory && trajectoryLive && (phase === 'rolling' || phase === 'scoring' || phase === 'turnEnd')
  // 一帧的步长按本次投掷的骰子数算（锁骰后只剩 N 颗，不是固定 42）
  const stride = Math.max(1, trajectoryDice || rollingCount || 6) * NUMBERS_PER_DIE

  // 始终渲染本次投掷的全部骰子（锁定只是隐藏，不卸载，保留物理状态与索引稳定）
  const count = rollingCount || 6

  // 已锁定的骰子从托盘隐藏：state.diceValues 是锁定后的剩余骰池，
  // 当前物理数组中未出现在该池里的骰子就是本轮刚锁定的骰子。
  const hiddenSet = useMemo(() => {
    if (phase !== 'scoring' && phase !== 'turnEnd') return new Set<number>()
    return hiddenIndices(settledValues, remainingDice ?? [])
  }, [phase, remainingDice, settledValues])

  // selected 是骰子索引集合
  const selectedSet = useMemo(() => new Set(selected), [selected])
  const selectableSet = useMemo(() => {
    if (!scoring || settledValues.some((v) => v == null)) return new Set<number>()
    const pool = settledValues.map((v, i) => (hiddenSet.has(i) ? null : v))
    return selectableIndices(pool, selected)
  }, [scoring, settledValues, hiddenSet, selected])

  return (
    <group>
      {Array.from({ length: count }, (_, i) => (
        <Die
          key={i}
          index={i}
          frames={trajectory}
          stride={stride}
          stepsPerFrame={stepsPerFrame}
          playing={playing}
          selectable={scoring && !hiddenSet.has(i) && (selectedSet.has(i) || selectableSet.has(i))}
          highlighted={selectedSet.has(i)}
          hidden={hiddenSet.has(i)}
        />
      ))}
      {lockedDice?.map((v, i) => (
        <LockedDie key={`L${i}`} value={v} index={i} opposite={!isMyTurn} />
      ))}
    </group>
  )
}

function CameraRig() {
  useFrame(({ camera, pointer }) => {
    // 轻微视差
    camera.position.x += (pointer.x * 0.5 - camera.position.x) * 0.03
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
        intensity={170}
        color="#ffc078"
        castShadow
        shadow-mapSize={[2048, 2048]}
        shadow-bias={-0.0004}
      />
      <ambientLight intensity={0.12} color="#4a3a55" />
      <Candle position={[6.6, 1.6, -4]} />

      {/* 固定步长 + 关闭插值：渲染状态直接来自模拟状态，两端画面一致 */}
      <Physics gravity={[0, -9.81, 0]} timeStep={PHYSICS_STEP} interpolate={false}>
        <Table />
        <DiceRig />
      </Physics>

      <TableDecor />
      <ContactShadows position={[0, -0.04, 0]} opacity={0.55} scale={22} blur={2.2} far={4} />
    </Canvas>
  )
}
