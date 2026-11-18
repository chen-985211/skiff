# 测试运行手册

依赖：Go 1.27.3+、aria2（macOS 可用 `brew install aria2`）；MyGo 版本锁定在 go.mod。

| 目的 | 命令 |
| --- | --- |
| 获取依赖 | `go mod download` |
| 格式化 | `gofmt -w cmd internal tests` |
| unit + 架构契约 | `go test ./...` |
| 目标 Red/Green | `go test ./tests/unit/transfer -run TestName -v` |
| 集成 | `go test -tags integration ./tests/integration/... -v` |
| 竞态 | `go test -race -tags integration ./...` |
| 静态检查 | `go vet ./...` |
| 编译 | `go build ./cmd/skiff` |
| 本机热重载（macOS 开发包使用 ad hoc 签名） | `make dev` |
| 单次开发运行 | `go run ./cmd/skiff` |
| 原生打包 | `go tool mygo build` |

真实下载测试只访问本机 HTTP 服务；不需要密钥或账户。若找不到 aria2，integration 应失败并解释依赖，不能静默跳过。基础版仅宣告当前实测的 macOS arm64，其他平台需原生验证。
