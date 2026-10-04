# 🎲 酒馆骰子 Farkle

媲美《天国：拯救》质感的 **3D 网页多人 Farkle（骰子）游戏**。

- **前端**：React + TypeScript + Vite + Three.js（@react-three/fiber）+ Rapier 物理引擎 + Zustand
- **后端**：Go + gorilla/websocket，服务端权威（crypto/rand 掷骰，防作弊）
- **材质**：程序化 Canvas PBR 贴图（胡桃木桌面、骨制骰子、皮革骰盅），暖色烛光 + ACES 色调映射
- **音效**：WebAudio 实时合成骰子碰撞声与酒馆氛围音，零音频文件

## 快速开始

### 本地开发

终端 1（后端，:8080）：

```bash
cd server
go run .
```

终端 2（前端，:5173）：

```bash
cd frontend
npm install
npm run dev
```

浏览器打开 http://localhost:5173 ，输入相同房间号即可多人同桌。

### Docker 一键部署

```bash
docker compose up --build
# 前端 http://localhost （nginx 反代 /ws → server:8080）
```

## 玩法规则（服务端权威判定）

- 6 颗骰，率先攒满 **10,000 分**者胜
- 单颗 1 = 100，单颗 5 = 50
- 三条：1→1000，n→n×100；四条/五条/六条依次 ×2 / ×4 / ×8
- 特殊：1-6 顺子 = 1500，三对 = 1500，两三条 = 2500
- 每次投掷必须锁定至少一颗得分骰，无可锁即 **Farkle**，本回合分清零
- **首次入库至少 300 分**
- 全部 6 颗锁定后可再掷 6 颗（经典规则）

### 回合操作

1. **锁定所选**：点击 3D 骰子选中（发光高亮），再点按钮锁定得分
2. **继续掷骰**：用剩余散骰再掷，累积回合分
3. **入库**：把回合分写入总分，回合结束

## 架构

```
┌────────────┐   WebSocket (JSON)   ┌──────────────────┐
│  React 3D  │ ◄──────────────────► │   Go Server      │
│  (R3F +    │   GAME_STATE         │  Hub → Room      │
│   Rapier)  │   DICE_ROLLED        │  权威计分 + RNG  │
└────────────┘   KEEP/BANK/ROLL     └──────────────────┘
```

- **结果预定投掷**：点数由 Go 服务端 `crypto/rand` 生成并广播；前端物理引擎真实投掷，骰子静止后平滑 slerp 校正到服务端指定的点数朝向上——兼顾物理真实感与防作弊
- 协议消息：`JOIN_ROOM` / `START_GAME` / `ROLL_DICE` / `KEEP_DICE` / `BANK_SCORE` / `GAME_STATE` / `DICE_ROLLED` / `ERROR`

## 测试

```bash
cd server
go test ./...                 # 计分规则单元测试
node e2e_test.js              # 双客户端端到端协议测试（需服务已启动，npm i ws）
```

## 目录

```
server/     Go WebSocket 服务端（hub / room / 计分引擎）
frontend/   React 3D 前端（scene / physics / UI / 合成音效）
```
