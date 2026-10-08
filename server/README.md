# Farkle Backend

## 目录结构

```text
server/
  main.go              # 仅负责配置、数据库、Hub、路由的装配
  config/              # YAML 配置加载与静态配置类型
  models/              # 协议消息、对局状态、WebSocket DTO、持久化模型
  game/                # 纯 Farkle 计分规则，不依赖传输和存储
  services/            # Hub、Room、Peer 抽象和权威对局状态机
  repositories/        # PostgreSQL / GORM 初始化与持久化入口
  routes/              # HTTP 路由、牌桌 WebSocket、大厅 WebSocket
  test/e2e/            # 手工端到端协议脚本
```

## 依赖方向

`config` 和 `models` 位于底层；`game` 只包含纯规则；`services` 组合规则与模型；`routes` 负责把 WebSocket 连接适配成 `services.Peer`；`main.go` 完成最终装配。

## 常用命令

```bash
go build ./...
go test ./...
go test -race ./...
```

WebSocket 端到端脚本见 `test/e2e/README.md`。
