import * as THREE from 'three'
import { Canvas, useFrame } from '@react-three/fiber'
import { ContactShadows, Environment } from '@react-three/drei'
import { CandleLight, Table } from './Table'
import { DiceRig } from './DiceRig'

/** 轻微视差：跟随鼠标的镜头微动 */
function CameraRig() {
  useFrame(({ camera, pointer }) => {
    camera.position.x += (pointer.x * 0.5 - camera.position.x) * 0.03
    camera.lookAt(0, 0, 0)
  })
  return null
}

/**
 * 场景装配：桌面 + 骰子 + 灯光。
 * 骰子完全由关键帧轨迹驱动，场景里没有物理世界。
 */
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
      <CandleLight position={[6.6, 1.6, -4]} />

      <Table />
      <DiceRig />
      <ContactShadows position={[0, -0.04, 0]} opacity={0.55} scale={22} blur={2.2} far={4} />
    </Canvas>
  )
}
