import { memo, useMemo, useRef, useState } from 'react'
import * as THREE from 'three'
import type { ThreeEvent } from '@react-three/fiber'
import { createDiceFaceTexture } from './textures'
import { useTrajectory } from './TrajectoryPlayer'
import { useGameStore } from '../store'

const DIE = 0.48 // 骰子边长，需与 trajectory.ts 保持一致

// BoxGeometry 材质面顺序：[+x, -x, +y, -y, +z, -z]
// 约定：右3 左4 上1 下6 前2 后5（对面之和为 7）
const FACE_VALUES = [3, 4, 1, 6, 2, 5]

interface DieProps {
  index: number
  /** 权威关键帧轨迹（扁平），null 表示本轮还没有轨迹 */
  frames: number[] | null
  /** 一帧的步长 = 本次投掷的骰子数 × 7 */
  stride: number
  stepsPerFrame: number
  playing: boolean
  selectable: boolean
  highlighted: boolean
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
  highlighted,
  hidden,
}: DieProps) {
  const group = useRef<THREE.Group>(null)
  const [hovered, setHovered] = useState(false)
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

  const emissive = highlighted ? '#7a5cff' : hovered && selectable ? '#3d5a80' : '#000000'
  const emissiveIntensity = highlighted ? 0.55 : hovered && selectable ? 0.25 : 0

  return (
    <group
      ref={group}
      visible={!hidden}
      // 没有轨迹时也摆在托盘里可见（避免"一颗骰子都没有"）
      position={[
        ((index % 3) - 1) * 1.6,
        DIE / 2,
        (Math.floor(index / 3) - 0.5) * 1.6,
      ]}
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
    </group>
  )
})
