import { useEffect } from 'react'
import { Scene } from './three/Scene'
import { HUD } from './ui/HUD'
import { Lobby } from './ui/Lobby'
import { playClick, playSelect, unlockAudio } from './audio'
import { useGameStore } from './store'

/** 按钮点击音 + 骰子选择音：全局挂一次即可 */
function useUiSounds() {
  useEffect(() => {
    const onPointerDown = (e: PointerEvent) => {
      // 首次手势顺便解锁 AudioContext（浏览器自动播放策略要求）
      unlockAudio()
      // 只有操作按钮（.btn）有点击音：选桌、发送等按钮保持安静
      const btn = (e.target as HTMLElement | null)?.closest('.btn')
      if (btn instanceof HTMLButtonElement && !btn.disabled) playClick()
    }
    document.addEventListener('pointerdown', onPointerDown, true)
    return () => document.removeEventListener('pointerdown', onPointerDown, true)
  }, [])

  useEffect(
    () =>
      useGameStore.subscribe((s, prev) => {
        if (s.selected === prev.selected) return
        // 只在选骰阶段发声，避免投掷开始时清空选择也响一声
        if (s.state?.phase !== 'scoring') return
        playSelect(s.selected.length > prev.selected.length)
      }),
    [],
  )
}

export default function App() {
  useUiSounds()
  return (
    <div className="app">
      <Scene />
      <HUD />
      <Lobby />
    </div>
  )
}
