import { Scene } from './three/Scene'
import { HUD } from './ui/HUD'
import { Lobby } from './ui/Lobby'

export default function App() {
  return (
    <div className="app">
      <Scene />
      <HUD />
      <Lobby />
    </div>
  )
}
