import { useMemo, useRef } from 'react'
import * as THREE from 'three'
import { useFrame } from '@react-three/fiber'
import { createFeltTexture, createWoodTexture } from './textures'
import { TRAY } from '../game/layout'

/** 火焰摇曳的点光源 */
export function CandleLight({
  position,
  color = '#ff9c3f',
  intensity = 6,
}: {
  position: [number, number, number]
  color?: string
  intensity?: number
}) {
  const ref = useRef<THREE.PointLight>(null)
  // 固定相位：不用 Math.random，保证两端看到同一套烛光闪烁
  const seed = 17.3
  useFrame(({ clock }) => {
    const t = clock.elapsedTime
    if (ref.current) {
      ref.current.intensity =
        intensity * (1 + Math.sin(t * 9 + seed) * 0.12 + Math.sin(t * 23 + seed * 2) * 0.06)
    }
  })
  return (
    <pointLight
      ref={ref}
      position={position}
      color={color}
      intensity={intensity}
      distance={12}
      decay={2}
      castShadow
      shadow-mapSize={[512, 512]}
    />
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

/**
 * 桌面与骰盘。骰子已经不再参与物理模拟，所以这里不再挂任何碰撞体，
 * 整个场景不需要 Physics 世界。
 */
export function Table() {
  const wood = useMemo(() => {
    const t = createWoodTexture()
    t.repeat.set(3, 2)
    return t
  }, [])
  const felt = useMemo(() => createFeltTexture(), [])

  return (
    <group>
      <mesh receiveShadow position={[0, -0.55, 0]}>
        <boxGeometry args={[26, 1, 16]} />
        <meshStandardMaterial map={wood} roughness={0.72} metalness={0.05} />
      </mesh>
      {/* 绿毡骰盘（与木桌明显区分） */}
      <mesh receiveShadow position={[0, -0.05, 0]}>
        <boxGeometry args={[TRAY.x * 2 + 1.6, 0.1, TRAY.z * 2 + 1.6]} />
        <meshStandardMaterial map={felt} roughness={0.95} metalness={0} />
      </mesh>
      {/* 桌面装饰：放在右上角，避开左上角的聊天框 */}
      <TableCandle position={[6.6, 0, -4]} />
    </group>
  )
}
