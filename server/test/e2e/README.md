# E2E scripts

这些脚本模拟真实 WebSocket 客户端，用于手工验证后端协议。它们不属于 Go 运行时或 CI。

先启动服务端，然后在本目录安装依赖并运行：

```bash
npm install
npm run protocol
npm run turns
```

默认连接 `ws://localhost:8086`，可通过 `FARKLE_WS_URL` 覆盖服务地址，例如：

```bash
FARKLE_WS_URL=ws://localhost:9000 npm run protocol
```
