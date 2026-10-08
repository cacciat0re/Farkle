import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import * as THREE from 'three'
import { useFrame } from '@react-three/fiber'
import { Die } from './Die'
import { createDiceFaceTexture } from './textures'
import { selectableIndices } from '../game/scoring'
import { hiddenIndices } from '../game/selection'
import { useGameStore, useIsMyTurn } from '../store'
import { NUMBERS_PER_DIE } from './trajectory'
import { DIE_SIZE, lockedSlotX, lockedSlotZ } from '../game/layout'

type Vec3 = [number, number, number]
type Quat = [number, number, number, number]
interface RestPose {
  p: Vec3
  q: Quat
}
interface FlyingDieItem {
  key: string
  value: number
  slot: number
  from: Vec3
  fromQuat: Quat
  start: number
}

function slotQuaternion(index: number, opposite: boolean) {
  const tilt = opposite ? Math.PI / 2.4 : -Math.PI / 2.4
  return new THREE.Quaternion().setFromEuler(new THREE.Euler(tilt, 0, 0.15 * index))
}

/** 已锁定的骰子：静态陈列在骰盘旁 */
function LockedDie({ value, index, opposite }: { value: number; index: number; opposite: boolean }) {
  const tex = useMemo(() => createDiceFaceTexture(value), [value])
  const q = useMemo(() => slotQuaternion(index, opposite), [index, opposite])
  return (
    <mesh castShadow position={[lockedSlotX(index), 0.23, lockedSlotZ(opposite)]} quaternion={q}>
      <boxGeometry args={[0.44, 0.44, 0.44]} />
      <meshStandardMaterial map={tex} roughness={0.32} />
    </mesh>
  )
}

const FLY_MS = 680
const _spin = new THREE.Quaternion()
const SPIN_AXIS = new THREE.Vector3(1, 0, 0)

/**
 * 锁定动画：从骰子在盘里的真实落点沿抛物线飞到陈列位。
 * 起点位置与朝向都取自轨迹末帧，终点按观看者自己那一侧计算，
 * 所以投掷者和观察者看到的是同一颗骰子飞向各自的方向。
 */
function FlyingDie({
  value,
  index,
  opposite,
  from,
  fromQuat,
  start,
  onDone,
}: {
  value: number
  index: number
  opposite: boolean
  from: Vec3
  fromQuat: Quat
  start: number
  onDone: () => void
}) {
  const group = useRef<THREE.Group>(null)
  const done = useRef(false)
  const tex = useMemo(() => createDiceFaceTexture(value), [value])
  const fromV = useMemo(() => new THREE.Vector3(from[0], from[1], from[2]), [from])
  const fromQ = useMemo(
    () => new THREE.Quaternion(fromQuat[0], fromQuat[1], fromQuat[2], fromQuat[3]).normalize(),
    [fromQuat],
  )
  const toV = useMemo(
    () => new THREE.Vector3(lockedSlotX(index), 0.23, lockedSlotZ(opposite)),
    [index, opposite],
  )
  const toQ = useMemo(() => slotQuaternion(index, opposite), [index, opposite])

  useFrame(() => {
    const g = group.current
    if (!g) return
    const t = Math.min(1, (performance.now() - start) / FLY_MS)
    const ease = 1 - Math.pow(1 - t, 3) // 起跳快、落地慢
    g.position.lerpVectors(fromV, toV, ease)
    g.position.y += Math.sin(Math.PI * t) * 0.75 // 抛物线弧顶
    // 空中翻一整圈，落地时正好对齐陈列角度
    _spin.setFromAxisAngle(SPIN_AXIS, Math.PI * 2 * (1 - ease))
    g.quaternion.copy(fromQ).slerp(toQ, ease).premultiply(_spin)
    // 飞完由动画自己通知父级移除，不用定时器
    if (t >= 1 && !done.current) {
      done.current = true
      onDone()
    }
  })

  return (
    <group ref={group} position={from} quaternion={fromQ}>
      <mesh castShadow>
        <boxGeometry args={[0.44, 0.44, 0.44]} />
        <meshStandardMaterial map={tex} roughness={0.32} />
      </mesh>
    </group>
  )
}

/** 骰子与锁定陈列：位姿完全来自服务端广播的关键帧轨迹 */
export function DiceRig() {
  const state = useGameStore((s) => s.state)
  const rollingCount = useGameStore((s) => s.rollingCount)
  const roll = useGameStore((s) => s.roll)
  const settledValues = useGameStore((s) => s.settledValues)
  const selected = useGameStore((s) => s.selected)
  const isMyTurn = useIsMyTurn()

  const { trajectory, trajectoryDice, trajectoryLive, stepsPerFrame } = roll
  const lockedDice = useMemo(() => state?.lockedDice ?? [], [state?.lockedDice])
  const remainingDice = useMemo(() => state?.diceValues ?? [], [state?.diceValues])
  const phase = state?.phase
  const scoring = phase === 'scoring' && isMyTurn
  const count = rollingCount || 6
  // 一帧的步长按本次投掷的骰子数算（锁骰后只剩 N 颗）
  const stride = Math.max(1, trajectoryDice || rollingCount || 6) * NUMBERS_PER_DIE
  const playing =
    !!trajectory && trajectoryLive && (phase === 'rolling' || phase === 'scoring' || phase === 'turnEnd')

  // 轨迹末帧 = 骰子真正停在哪儿：锁定动画的起点与桌面圆环的位置都取自这里
  const restPoses = useMemo<RestPose[] | null>(() => {
    if (!trajectory || stride <= 0) return null
    const total = Math.floor(trajectory.length / stride)
    if (total < 1) return null
    const base = (total - 1) * stride
    const poses: RestPose[] = []
    for (let i = 0; i * NUMBERS_PER_DIE + 6 < stride; i++) {
      const b = base + i * NUMBERS_PER_DIE
      poses.push({
        p: [trajectory[b], trajectory[b + 1], trajectory[b + 2]],
        q: [trajectory[b + 3], trajectory[b + 4], trajectory[b + 5], trajectory[b + 6]],
      })
    }
    return poses
  }, [trajectory, stride])

  // 换回合/重投时轨迹会被清空，这里留住上一投的落点，供锁定动画当起点
  const lastRestPoses = useRef<RestPose[]>([])
  useEffect(() => {
    if (restPoses) lastRestPoses.current = restPoses
  }, [restPoses])

  // 已锁定的骰子从骰盘隐藏：state.diceValues 是锁定后的剩余骰池
  const hiddenSet = useMemo(() => {
    if (phase !== 'scoring' && phase !== 'turnEnd') return new Set<number>()
    return hiddenIndices(settledValues, remainingDice)
  }, [phase, remainingDice, settledValues])

  // 选中的骰子：用服务端广播的 selection（自己的选择本地先更新，再被回显覆盖）
  const selectedSet = useMemo(() => new Set(state?.selection ?? []), [state?.selection])
  const selectableSet = useMemo(() => {
    if (!scoring || settledValues.some((v) => v == null)) return new Set<number>()
    const pool = settledValues.map((v, i) => (hiddenSet.has(i) ? null : v))
    return selectableIndices(pool, selected)
  }, [scoring, settledValues, hiddenSet, selected])

  // 新锁定的骰子：从盘里的落点跳到陈列位
  const [flying, setFlying] = useState<FlyingDieItem[]>([])
  const prevLockedRef = useRef(0)
  useEffect(() => {
    const prev = prevLockedRef.current
    prevLockedRef.current = lockedDice.length

    // 换人或热骰会清空锁定骰子：把还在飞的动画一起收掉，
    // 否则旧骰子会一直留在桌上，显示上一轮的状态
    if (lockedDice.length < prev) {
      setFlying([])
      return
    }
    if (lockedDice.length === prev) return

    // 托盘索引与锁定骰子的对应：按点数贪心匹配（与服务端扣池顺序一致）
    const used = new Set<number>()
    const matchTray = (value: number) => {
      for (let i = 0; i < settledValues.length; i++) {
        if (settledValues[i] === value && !used.has(i)) {
          used.add(i)
          return i
        }
      }
      return -1
    }
    for (const v of lockedDice.slice(0, prev)) matchTray(v)

    const poses = lastRestPoses.current
    const start = performance.now()
    const items = lockedDice.slice(prev).map((value, k) => {
      const slot = prev + k
      const pose = poses[matchTray(value)]
      return {
        key: `${slot}-${value}-${start}`,
        value,
        slot,
        from: pose?.p ?? ([lockedSlotX(slot), DIE_SIZE / 2, 1.2] as Vec3),
        fromQuat: pose?.q ?? ([0, 0, 0, 1] as Quat),
        start,
      }
    })
    setFlying((f) => [...f, ...items])
  }, [lockedDice, settledValues])

  const removeFlying = useCallback((key: string) => {
    setFlying((f) => f.filter((x) => x.key !== key))
  }, [])

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
          hidden={hiddenSet.has(i)}
        />
      ))}

      {/* 选中标记：贴在桌面上的圆环，水平于桌面，不跟随骰子自身朝向 */}
      {[...selectedSet].map((i) => {
        if (hiddenSet.has(i)) return null
        const pose = restPoses?.[i]
        if (!pose) return null
        return (
          <mesh key={`ring-${i}`} position={[pose.p[0], 0.02, pose.p[2]]} rotation={[-Math.PI / 2, 0, 0]}>
            <ringGeometry args={[0.33, 0.44, 44]} />
            <meshBasicMaterial
              color="#ffd479"
              transparent
              opacity={0.9}
              side={THREE.DoubleSide}
              depthWrite={false}
            />
          </mesh>
        )
      })}

      {lockedDice.map((v, i) =>
        flying.some((f) => f.slot === i) ? null : (
          <LockedDie key={`L${i}`} value={v} index={i} opposite={!isMyTurn} />
        ),
      )}
      {flying.map((f) => (
        <FlyingDie
          key={f.key}
          value={f.value}
          index={f.slot}
          opposite={!isMyTurn}
          from={f.from}
          fromQuat={f.fromQuat}
          start={f.start}
          onDone={() => removeFlying(f.key)}
        />
      ))}
    </group>
  )
}
