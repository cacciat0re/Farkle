# 酒馆骰子 Farkle

中世纪酒馆风格的 3D 多人 Farkle 骰子游戏。

- **前端**：React + TypeScript + Vite + Three.js + Rapier + Zustand
- **后端**：Go + chi + Gorilla WebSocket + GORM + PostgreSQL
- **画面**：程序化木桌、绿毡骰盘、骨制骰子、烛光、软阴影、音效实时合成
- **网络**：服务端权威状态机；客户端只上报物理动画停稳后的骰子点数
- **实时协议**：大厅使用 `/ws/lobby` 推送桌位变化，游戏桌使用 `/ws/{tableId}`；两者都是自定义 JSON 消息，不依赖 STOMP broker

## 环境要求

- Node.js 20+
- Go 1.26+
- PostgreSQL 16+，本机 `localhost:5432`

数据库连接和大厅桌子列表都在 [`server/config.yaml`](server/config.yaml) 中配置：

```yaml
database:
  dsn: "host=localhost user=postgres password=20230612 dbname=farkle port=5432 sslmode=disable"

tables:
  - id: apple
    name: Apple
    max_players: 2
```

首次运行前创建数据库：

```bash
createdb -h localhost -U postgres farkle
```

服务启动时会自动迁移 `users` 和 `match_records` 表，供后续 OIDC 与战绩功能使用。当前游客只在页面会话中临时存在：不写浏览器持久化存储，也不会写入数据库。

## 本地开发

终端 1，启动后端：

```bash
cd server
go run .
```

终端 2，启动前端：

```bash
cd frontend
npm install
npm run dev
```

打开 <http://localhost:5173>。页面会话会自动生成一个临时游客身份，刷新后重新生成；大厅展示 `config.yaml` 中的六张桌子，并通过 WebSocket 实时接收人数和开局状态变化。当前阶段每桌固定两个座位、禁止中途加入和观战，必须两名玩家都点击准备后才会自动开局。

## 玩法

- 6 颗骰，率先达到 **3,000 分**者获胜
- 单颗 1 = 100，单颗 5 = 50
- 三条：1 → 1000，n → n×100；四条/五条/六条继续翻倍
- 特殊组合：1-6 顺子 = 1500，三对 = 1500，两三条 = 2500
- 锁定组合中的每颗骰子必须参与得分，不能夹带废骰
- 无可锁组合即 **Farkle**，本回合分清零
- 首次入库至少 300 分
- 全部骰子锁定后，下一次可重新投掷 6 颗

点击骰子进行选择。只有能够组成合法得分组合的骰子会响应选择；锁定后系统会显示正确总分。

终局后每名玩家可以独立选择继续游戏或退出到大厅。两人都选择继续后，房间回到等待状态，分数清零，双方再次点击准备后开始下一局。

## 测试

```bash
cd server
go test ./...
node e2e_test.js
node e2e_turns_test.js
```

前端构建检查：

```bash
cd frontend
npm run build
```

E2E 测试要求后端正在 `:8080` 运行，并使用 `Apple` 与 `Butter` 两张测试桌。

## 目录

```text
server/
  main.go        chi 路由、REST API、WebSocket 入口
  hub.go         固定桌子集合与大厅状态
  room.go        房间状态机和回合流程
  game.go        Farkle 计分与严格锁定校验
  db.go          GORM 模型和数据库操作
  config.yaml    数据库与桌子配置

frontend/
  src/ui/        Lobby 与游戏 HUD
  src/three/     Three.js / Rapier 场景和骰子
  src/game/      客户端计分镜像
  src/store.ts   WebSocket 与本地游戏状态
```
