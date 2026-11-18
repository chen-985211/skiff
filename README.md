# Skiff

基于 **MyGo 原生 UI + aria2** 的桌面下载工具，当前版本 **0.1.0**。

## 基础版功能

- 批量添加 HTTP/HTTPS 链接，每行一个；校验整批链接并去重。
- 点左下角文件夹打开保存目录。在「设置」中可以指定默认保存目录、开关背景流光。单次下载可单独选择目录。
- 最多同时下载 3 个任务，单服务器最多 4 个连接。
- 任务卡片显示状态、进度、速度与失败原因。点击文件名进入详情，查看来源链接和本地位置。底部操作栏可打开新建下载面板。
- 支持暂停、继续和失败重试。按状态、名称或链接筛选后，可对当前结果批量暂停或继续。
- 下载源不支持续传时明确提示；用户确认后以新文件名单连接从头下载，保留原来的部分文件。
- 移除任务保留文件；在文件夹中显示已下载文件。
- 任务与历史保存在本地；重启后未完成任务以暂停状态恢复。
- 跟随系统明暗主题和减少动态效果设置。`⌘N` / `Ctrl+N` 新建下载，`⌘F` / `Ctrl+F` 搜索。关闭新建窗口后，本次运行中可继续编辑链接和保存目录。

首版不包含 BT/磁力、浏览器下载接管、限速设置或后台托盘。

## 开发运行

需要 Go **1.27.3+** 和 aria2。Go 可按 go.mod 自动选择所需工具链。

```sh
brew install aria2
go mod download
make dev
```

`make dev` 会在代码变化后重新构建并启动开发包。macOS 开发包使用本机 ad hoc 签名。直接运行 `go tool mygo dev` 会让 MyGo 自行选择钥匙串中的开发证书。按 Ctrl+C 停止热重载。

aria2 作为 Skiff 管理的独立进程运行，无需手动启动 RPC 服务。本版不内置 aria2 二进制；打包后的应用仍需要本机安装 aria2。应用会查找 PATH、Homebrew 常用路径，以及资源目录中的 `engine/aria2c`。

## 验证与构建

```sh
make test                 # unit + 架构契约
make test-integration     # 真实 aria2、本地 HTTP、RPC 与文件仓储
make check                # 格式 + vet + 全部测试（含 race）+ 编译
make package              # 当前平台的原生应用包
```

macOS 应用在 `build/darwin-arm64/Skiff.app`。MyGo 本地打包采用 ad hoc 签名，尚未进行 Developer ID 签名与 Apple 公证。

## 许可证

Skiff 采用 [MIT 许可证](LICENSE)。
