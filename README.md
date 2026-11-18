# 轻舟 · Skiff

基于 **MyGo 原生 UI + aria2** 的桌面下载工具，当前版本 **0.1.1**。

## 基础版功能

- 批量添加 HTTP/HTTPS 链接，每行一个；校验整批链接并去重。
- 选择保存目录；最多同时下载 3 个任务，单服务器最多 4 个连接。
- 任务卡片显示状态、进度、速度与失败原因。点击文件名进入详情，查看来源链接和本地位置。底部操作栏可打开新建下载面板。
- 支持暂停、继续和失败重试。按状态、名称或链接筛选后，可对当前结果批量暂停或继续。
- 下载源不支持续传时明确提示；用户确认后以新文件名单连接从头下载，保留原来的部分文件。
- 移除任务保留文件；在文件夹中显示已下载文件。
- 任务与历史保存在本地；重启后未完成任务以暂停状态恢复。
- 跟随系统明暗主题和减少动态效果设置。`⌘N` / `Ctrl+N` 新建下载，`⌘F` / `Ctrl+F` 搜索。关闭新建窗口后，本次运行中可继续编辑链接和保存目录。

当前已在 **macOS arm64** 验证。Windows 和 Linux 尚未验收。首版不包含 BT/磁力、浏览器下载接管、限速设置或后台托盘。

## 开发运行

需要 Go **1.27.3+** 和 aria2。Go 可按 go.mod 自动选择所需工具链。

```sh
brew install aria2
go mod download
make dev
```

`make dev` 会在代码变化后重新构建并启动开发包。macOS 开发包使用本机 ad hoc 签名。直接运行 `go tool mygo dev` 会让 MyGo 自行选择钥匙串中的开发证书。按 Ctrl+C 停止热重载。

aria2 作为轻舟管理的独立进程运行，无需手动启动 RPC 服务。本版不内置 aria2 二进制；打包后的应用仍需要本机安装 aria2。应用会查找 PATH、Homebrew 常用路径，以及资源目录中的 `engine/aria2c`。

## 验证与构建

```sh
make test                 # unit + 架构契约
make test-integration     # 真实 aria2、本地 HTTP、RPC 与文件仓储
make check                # 格式 + vet + 全部测试（含 race）+ 编译
make package              # 当前平台的原生应用包
```

macOS 应用在 `build/darwin-arm64/轻舟.app`。MyGo 本地打包采用 ad hoc 签名，尚未进行 Developer ID 签名与 Apple 公证。

## 架构与 TDD

以 `/Users/nature/Development/cleancode` 的 DDD + Clean Architecture 为架构参考，采用上下文优先、应用层端口和依赖向内原则。

```text
cmd/skiff/
internal/contexts/transfer/
  domain/                     任务聚合、链接与状态规则
  application/                用例、端口和 DTO
  infrastructure/
    aria2/                    JSON-RPC、受管子进程
    persistence/              原子文件仓储
  presentation/               MyGo 界面和交互状态
internal/platform/desktop/    装配、系统能力和应用生命周期
tests/
  unit/transfer/
  integration/transfer/
  contract/
```

详见 [架构](docs/engineering/architecture.md)、[测试运行手册](docs/testing/TESTING_RUNBOOK.md) 和 [TDD 记录](docs/testing/TDD_LOG.md)。

## 数据与诊断

macOS 默认数据目录为 `~/Library/Application Support/Skiff/`，默认下载目录为 `~/Downloads/Skiff/`。

- `tasks.json`：版本化任务与历史，采用私有权限、临时文件和原子替换。
- `aria2.session`：下载引擎恢复资料。
- `aria2.conf`：本次运行的独立 RPC 配置，仅监听本机回环地址，使用随机密钥。
- `engine.log`：当前运行的引擎错误日志。

任务文件可能包含带签名或授权参数的下载 URL，请勿将真实数据目录提交到仓库。

| 环境变量 | 用途 |
| --- | --- |
| `SKIFF_DATA_DIR` | 指定独立数据目录，适合开发与测试 |
| `SKIFF_ARIA2_BINARY` | 指定 aria2c 路径 |

集成测试仅访问本机 HTTP 服务，并使用独立临时目录，不读取真实任务数据。

## 关于暂停后无法继续

断点续传要求下载源正确响应 HTTP Range。某些动态测速地址（例如 Cloudflare 的 `__down`）会忽略区间请求并返回整份数据；aria2 因此以错误码 8 拒绝续传，最终错误文案可能仅为 `No URI available.`。

0.1.1 按结构化错误码识别该情况，显示明确原因，并将普通重试替换为「从头下载」。确认后会生成带 `.restart-<唯一标识>` 的不同文件名，并使用单连接完整下载。原文件不会被删除或覆盖。此类下载再次暂停后仍可能需要从头下载，这是下载源的限制。
