<p align="center">
  <img src="resources/icon.png" width="112" height="112" alt="Skiff Logo">
</p>

<h1 align="center">Skiff</h1>

<p align="center">基于 <strong>MyGo 原生 UI + aria2</strong> 的桌面下载工具，当前版本 <strong>0.1.0</strong>。</p>

<p align="center">
  <a href="https://github.com/chen-985211/skiff/actions/workflows/ci.yml"><img src="https://github.com/chen-985211/skiff/actions/workflows/ci.yml/badge.svg?branch=main" alt="CI 状态"></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%C2%B7%20Windows%20%C2%B7%20Linux-52647A" alt="平台：macOS、Windows、Linux">
  <a href="https://github.com/aria2/aria2"><img src="https://img.shields.io/badge/engine-aria2-366C74" alt="下载引擎：aria2"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-626B82" alt="MIT 许可证"></a>
</p>

<p align="center"><a href="https://github.com/chen-985211/skiff/issues/1">观看 Skiff 下载演示（15 秒）</a></p>

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

## 首次使用

在 Mac 上打开 Skiff。如果底部显示「下载引擎」，点击一次即可。Skiff 会按芯片架构从 GitHub 下载 aria2c，校验后安装到当前用户的应用数据目录。按钮恢复为「新建下载」后即可添加链接。已安装可用 aria2 的用户可以直接新建下载。

下载使用 Motrix 项目维护的 [aria2c 1.37.0-motrix.24 发布包](https://github.com/motrixapp/aria2/releases/tag/v1.37.0-motrix.24)，支持 Apple Silicon 和 Intel。首次安装需要能访问 GitHub。

## 开发运行

需要 Go **1.27.3+**。Go 可按 go.mod 自动选择所需工具链。

```sh
go mod download
make dev
```

`make dev` 会在代码变化后重新构建并启动开发包。macOS 开发包使用本机 ad hoc 签名。直接运行 `go tool mygo dev` 会让 MyGo 自行选择钥匙串中的开发证书。按 Ctrl+C 停止热重载。

aria2 由 Skiff 作为独立进程启动。运行集成测试时，可使用 `brew install aria2` 准备测试引擎，或用 `SKIFF_ARIA2_BINARY` 指向已有的 aria2c。

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
