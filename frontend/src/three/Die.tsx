import { memo, useEffect, useMemo, useRef } from 'react'
import * as THREE from 'three'
import type { ThreeEvent } from '@react-three/fiber'
import { createDiceFaceTexture } from './textures'
import { useTrajectory } from './TrajectoryPlayer'
import { useGameStore } from '../store'
import { DIE_SIZE, DIE_STASH, FACE_VALUES } from '../game/layout'

const DIE = DIE_SIZE

interface DieProps {
  index: number
  /** 权威关键帧轨迹（扁平），null 表示本轮还没有轨迹 */
  frames: number[] | null
  /** 一帧的步长 = 本次投掷的骰子数 × 7 */
  stride: number
  stepsPerFrame: number
  playing: boolean
  selectable: boolean
  hidden: boolean
}

/**
 * 骰子已不再参与本地物理：位姿完全来自服务端广播的同一份关键帧轨迹，
 * 因此所有客户端（含投掷者）看到的过程与终点完全一致。
 */
export const Die = memo(function Die({
  index,
  frames,
  stride,
  stepsPerFrame,
  playing,
  selectable,
  hidden,
}: DieProps) {
  const group = useRef<THREE.Group>(null)
  const toggleSelect = useGameStore((s) => s.toggleSelect)

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

  useTrajectory(group, frames, stride, stepsPerFrame, index, playing)

  // 本轮还没有轨迹（等待中 / 刚换回合 / 刚点了继续投掷）时把骰子收回桌面下方，
  // 否则上一轮的骰子会留在桌上，看起来像"没刷新"
  useEffect(() => {
    const obj = group.current
    if (!obj || frames) return
    obj.position.set(DIE_STASH[0], DIE_STASH[1], DIE_STASH[2])
  }, [frames])

  useEffect(
    () => () => {
      document.body.style.cursor = ''
    },
    [],
  )

  return (
    <group
      ref={group}
      visible={!hidden}
      // 初始就收在桌面下方；有轨迹后由 useTrajectory 接管位姿
      position={DIE_STASH}
    >
      <mesh
        castShadow
        receiveShadow
        onClick={(e: ThreeEvent<MouseEvent>) => {
          if (!selectable) return
          e.stopPropagation()
          toggleSelect(index)
        }}
        onPointerOver={(e: ThreeEvent<PointerEvent>) => {
          if (selectable) {
            e.stopPropagation()
            document.body.style.cursor = 'pointer'
          }
        }}
        onPointerOut={() => {
          document.body.style.cursor = ''
        }}
      >
        <boxGeometry args={[DIE, DIE, DIE]} />
        {materials.map((m, i) => (
          <primitive key={i} object={m} attach={`material-${i}`} />
        ))}
      </mesh>
    </group>
  )
})
