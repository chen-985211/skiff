<p align="center">
  <img src="resources/icon.png" width="112" height="112" alt="Skiff Logo">
</p>

<h1 align="center">Skiff</h1>

<p align="center">基于 <strong>MyGo 原生 UI + aria2</strong> 的桌面下载工具，当前版本 <strong>0.1.0</strong>。</p>

<p align="center">
  <a href="https://github.com/chen-985211/skiff/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/chen-985211/skiff/ci.yml?branch=main&amp;label=CI&amp;labelColor=374357" alt="CI 状态"></a>
  <a href="https://github.com/chen-985211/skiff/releases"><img src="https://img.shields.io/badge/platform-macOS%20%C2%B7%20Windows%20%C2%B7%20Linux-496C99?labelColor=374357" alt="平台：macOS、Windows、Linux"></a>
  <a href="https://github.com/aria2/aria2"><img src="https://img.shields.io/badge/engine-aria2-32766F?labelColor=374357" alt="下载引擎：aria2"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-796194?labelColor=374357" alt="MIT 许可证"></a>
</p>

<div align="center">

https://github.com/user-attachments/assets/8fa22cc3-e91a-4537-87f4-756609c0314c

</div>

## 基础版功能

- 批量添加 HTTP/HTTPS 或 FTP 文件链接，每行一个；逐行校验，同批及同目录未结束任务去重。有效链接入队，无效行和超过每批 100 条的链接保留供修改或再次提交。
- 点左下角文件夹打开保存目录。在「设置」中可以指定默认保存目录、开关背景流光。单次下载可单独选择目录。
- 默认同时下载 10 个任务，可在设置中调整为 1～200 个并保存。队列自动补位，等待任务可选择「优先下载」占用下一个空位。新建任务默认自动分段，先检查文件大小和实际 Range 响应；小文件或无法确认分段能力时用单连接，自动模式最多 4 路。在「高级选项」可设 1～16 路上限，下载详情可查看实际连接数。
- 任务卡片显示状态、进度、速度与失败原因。点击文件名进入详情，查看来源链接和本地位置。底部操作栏可打开新建下载面板。
- 支持暂停、继续和失败重试。按状态、名称或链接筛选后，可对当前结果批量暂停或继续。
- HTTP/HTTPS 下载跟随重定向并识别服务端文件名。续传优先核对强 ETag 或可用的最后修改时间。缺少版本标识时，直接按原检查点续传。发现内容已变或来源拒绝分段时保留原文件。
- 下载源不支持续传时明确提示；用户确认后以新文件名单连接从头下载，保留原来的部分文件。
- 移除任务保留文件；在文件夹中显示已下载文件。
- 任务与历史保存在本地；重启后未完成任务以暂停状态恢复。
- 跟随系统明暗主题和减少动态效果设置。`⌘N` / `Ctrl+N` 新建下载，`⌘F` / `Ctrl+F` 搜索。关闭新建窗口后，本次运行中可继续编辑链接和保存目录。

可在设置中调整全局下载上限，在任务详情单独限速，默认不限速。代理默认跟随系统，可对新任务设置直连、带认证的 HTTP 代理、绕过列表和 DNS 解析模式，单任务支持覆盖代理。

浏览器复制的 cURL 可直接粘贴到新建下载。新建下载的高级选项把文件与连接、请求头、代理放在同一面板，可直接编辑后开始下载。「保存到」可切换最近使用的目录。所有下载目标与验收入口见 [普通文件下载目标](docs/product/file-download-goals.md)。

首版不包含 BT/磁力、浏览器下载接管或后台托盘。

## 首次使用

### macOS 首次打开

1. 在配备 Apple 芯片（M 系列）的 Mac 上，下载 [Skiff 的 macOS 安装包（DMG）][macos-dmg]，打开后将 **Skiff.app** 拖到「应用程序」。
2. 在「应用程序」中双击 **Skiff**。此版本使用临时签名（ad hoc），尚未经过 Apple 公证。如果 macOS 提示无法验证开发者，请先关闭提示。
3. 确认应用来自上面的官方 Release 后，进入 **系统设置 → 隐私与安全性**，找到 Skiff，点 **「仍要打开」** 并确认。以后就可以直接双击启动。[Apple 官方说明](https://support.apple.com/zh-cn/102445)

**也可以用终端完成首次打开。** 将官方 DMG 中的应用拖到「应用程序」后，运行下面这行命令。它会移除这份 Skiff 的下载隔离标记，然后启动应用。

```sh
xattr -dr com.apple.quarantine /Applications/Skiff.app && open /Applications/Skiff.app
```

### 开始下载

打开 Skiff 后，如果底部显示「下载引擎」，点击一次即可。Skiff 会按芯片架构从 GitHub 下载 aria2c，校验后安装到当前用户的应用数据目录。按钮恢复为「新建下载」后即可添加链接。已安装可用 aria2 的用户可以直接新建下载。

下载使用 Motrix 项目维护的 [aria2c 1.37.0-motrix.24 发布包](https://github.com/motrixapp/aria2/releases/tag/v1.37.0-motrix.24)，支持 Apple Silicon 和 Intel。首次安装需要能访问 GitHub。

[macos-dmg]: https://github.com/chen-985211/skiff/releases/download/v0.1.0/skiff-0.1.0-macos-arm64.dmg

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

推送与 `mygo.json` 版本一致的标签（例如 `v0.1.0`）即可触发发布。三个平台的测试与打包全部通过后，[GitHub Releases](https://github.com/chen-985211/skiff/releases) 会提供 macOS Apple Silicon 的 DMG、Windows x64 的 EXE，以及 Linux x64 的 DEB、RPM 和 tar.gz。

## 许可证

Skiff 采用 [MIT 许可证](LICENSE)。
