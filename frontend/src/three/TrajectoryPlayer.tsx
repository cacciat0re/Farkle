import { useEffect, useRef, type RefObject } from 'react'
import * as THREE from 'three'
import { useFrame } from '@react-three/fiber'
import { NUMBERS_PER_DIE, PHYSICS_DT } from './trajectory'

const _p0 = new THREE.Vector3()
const _p1 = new THREE.Vector3()
const _q0 = new THREE.Quaternion()
const _q1 = new THREE.Quaternion()

function applyFrame(obj: THREE.Object3D, frames: number[], stride: number, index: number, frame: number) {
  const base = frame * stride + index * NUMBERS_PER_DIE
  obj.position.set(frames[base], frames[base + 1], frames[base + 2])
  obj.quaternion
    .set(frames[base + 3], frames[base + 4], frames[base + 5], frames[base + 6])
    .normalize()
}

/**
 * 用同一份关键帧驱动骰子：位置 lerp、四元数 slerp。
 * 播放进度按真实时间推进，与物理步进/设备刷新率完全解耦。
 *
 * 关键点：
 * - stride 随本次投掷的骰子数变化（锁骰后只剩 N 颗），不能写死 6×7；
 * - 只有"本轮开局时在线"的客户端才从第 0 帧播动画，
 *   中途补看的人直接定位到末帧，避免画面莫名其妙重播一遍。
 */
export function useTrajectory(
  target: RefObject<THREE.Object3D | null>,
  frames: number[] | null | undefined,
  stride: number,
  stepsPerFrame: number,
  index: number,
  playing: boolean,
) {
  const head = useRef(0)
  const playingRef = useRef(playing)
  // 用 effect 同步"是否播放"，这样"轨迹到达"的 effect 只在 frames 变化时跑一次，
  // 不会因为阶段切换（scoring → turnEnd 等）把播放头重置、导致动画重播。
  useEffect(() => {
    playingRef.current = playing
  }, [playing])

  // 轨迹到达时定好播放头：要播放就回开头，否则直接摆到末帧（权威终态）
  useEffect(() => {
    const obj = target.current
    if (!obj || !frames || stride <= 0 || frames.length < stride) return
    const total = Math.floor(frames.length / stride)
    if (total < 1) return
    head.current = playingRef.current ? 0 : total - 1
    applyFrame(obj, frames, stride, index, head.current)
  }, [frames, stride, index, target])

  useFrame((_, dt) => {
    const obj = target.current
    if (!obj || !frames || !playing || stride <= 0) return
    const total = Math.floor(frames.length / stride)
    if (total < 2) return

    const last = total - 1
    if (head.current >= last) {
      applyFrame(obj, frames, stride, index, last)
      return
    }

    // 每帧推进量 = 真实秒数 / 每关键帧代表的秒数
    const framesPerSecond = 1 / (stepsPerFrame * PHYSICS_DT)
    head.current = Math.min(last, head.current + dt * framesPerSecond)

    const f0 = Math.floor(head.current)
    const f1 = Math.min(last, f0 + 1)
    const a = head.current - f0
    const b0 = f0 * stride + index * NUMBERS_PER_DIE
    const b1 = f1 * stride + index * NUMBERS_PER_DIE

    _p0.set(frames[b0], frames[b0 + 1], frames[b0 + 2])
    _p1.set(frames[b1], frames[b1 + 1], frames[b1 + 2])
    obj.position.lerpVectors(_p0, _p1, a)

    _q0.set(frames[b0 + 3], frames[b0 + 4], frames[b0 + 5], frames[b0 + 6])
    _q1.set(frames[b1 + 3], frames[b1 + 4], frames[b1 + 5], frames[b1 + 6])
    // slerp 内部会做最短路径处理
    obj.quaternion.slerpQuaternions(_q0, _q1, a)
  })
}
