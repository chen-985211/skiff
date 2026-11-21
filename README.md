<p align="center">
  <img src="resources/icon.png" width="112" height="112" alt="Skiff Logo">
</p>

<h1 align="center">Skiff</h1>

<p align="center">基于 MyGo 构建的桌面下载工具，采用 Go 原生 UI 和 GPU 渲染，支持文件直链、磁力链接和种子文件。</p>

<p align="center">
  <a href="https://github.com/chen-985211/skiff/releases/latest"><img src="https://img.shields.io/github/v/release/chen-985211/skiff?label=version&amp;color=blue" alt="Skiff 最新正式版"></a>
  <a href="https://github.com/chen-985211/skiff/releases"><img src="https://img.shields.io/badge/platform-macOS%20%C2%B7%20Windows%20%C2%B7%20Linux-lightgrey" alt="平台：macOS、Windows、Linux"></a>
  <a href="https://github.com/aria2/aria2"><img src="https://img.shields.io/badge/engine-aria2-orange" alt="下载引擎：aria2"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-brightgreen" alt="MIT 许可证"></a>
</p>

<div align="center">

https://github.com/user-attachments/assets/8fa22cc3-e91a-4537-87f4-756609c0314c

</div>

## 下载功能

### 普通文件

- **添加链接**　在「新建下载 → 链接」粘贴 HTTP、HTTPS、FTP 文件地址，或浏览器复制的 cURL。批量添加时每行一个链接，每批最多 100 条。有效链接先入队，重复链接自动跳过，无效及超出上限的行留在输入框中。
- **保存与续传**　可使用默认目录、切换最近目录或为单个文件改名。HTTP 下载跟随重定向并识别服务端文件名。暂停后从断点继续，缺少版本标识时直接使用原检查点。如果远端文件已变或服务器拒绝续传，可另存新文件从头下载，已下载的部分会保留。
- **队列与连接**　默认同时下载 10 项，可调至 1～200 项。队列满时，可将等待任务设为「优先下载」。单文件默认按大小和服务器的分段能力自动选择 1～4 路连接，也可在「高级选项」指定 1～16 路上限，实际连接数可在详情中查看。
- **限速与网络**　在「设置 → 全局限速」控制总带宽，也可在任务详情单独限速。两处都支持预设和自定义数值，运行中即可生效。「代理与 DNS」可设置系统代理、直连、带认证的 HTTP 代理、绕过列表和 DNS 模式。
- **高级选项**　可为单个任务填写备用下载地址、请求头或代理。主地址失败时依次尝试备用地址，确认内容一致后复用进度。「设置 → 性能与磁盘」可调整缓存、最小分段和文件预分配。

### BitTorrent 与磁力

- **选文件下载**　在「新建下载 → BT」打开本地 `.torrent`，或粘贴磁力链接并点「读取文件列表」。在文件树中勾选需要的文件，再点「开始下载」。磁力任务可选 30、60 或 120 秒后自动下载全部，手动更改勾选会取消倒计时。
- **管理节点**　在「设置 → BT 网络」管理 Tracker 订阅、DHT、局域网节点发现、监听端口和节点上限。任务详情可查看节点、Tracker 和分片进度。
- **做种与复用文件**　下载完成后可继续做种，并按分享率或时长停止。已有完整文件可在任务详情中校验、重新定位或移动，再恢复做种。
- **创建种子**　在「BT 选项 → 创建种子…」选择本地文件或文件夹，生成 `.torrent` 文件。勾选「用原文件做种」即可在保存后开始分享。

### 任务与操作

- 任务卡片显示进度、速度和失败原因。点击文件名可查看来源和本地位置，点左下角文件夹可打开保存目录。
- 可暂停、继续或重试，也可按状态、名称或链接筛选后批量暂停、继续。重启后，未完成的下载以暂停状态恢复。移除任务会保留磁盘上的文件。
- `⌘N` / `Ctrl+N` 新建下载，`⌘F` / `Ctrl+F` 搜索。同一次运行中，关闭新建面板再打开可继续编辑链接和保存目录。
- 限速、BT 选项和设置共用同一款下拉菜单。可用 ↑ / ↓ 移动高亮、Enter 确认、Esc 收起。快速反向时，高亮从当前位置继续移动。开启系统「减少动态效果」后，高亮直接切换。明暗主题和背景流光可在「设置 → 外观」中调整。

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

打开 Skiff，点窗口底部的「新建下载」。如果显示「下载引擎」，点一次即可。Skiff 会按系统和处理器架构下载并校验 aria2c，完成后即可新建下载。macOS、Windows 和 Linux 均支持 x64 和 ARM64，首次下载引擎需要访问 GitHub。[Skiff 使用的 aria2c 发布包](https://github.com/motrixapp/aria2/releases/tag/v1.37.0-motrix.24)

[macos-dmg]: https://github.com/chen-985211/skiff/releases/download/v0.1.0/skiff-0.1.0-macos-arm64.dmg

## 开发运行

安装 Go 后，在项目目录运行以下命令。Go 会按 [go.mod](go.mod) 自动选择所需工具链。

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
make check-ui             # 界面契约、交互和动效回归
make check                # 格式 + vet + 全部测试（含 race）+ 编译
make package              # 当前平台的原生应用包
```

macOS 应用在 `build/darwin-arm64/Skiff.app`。MyGo 本地打包采用 ad hoc 签名，尚未进行 Developer ID 签名与 Apple 公证。

推送以 `v` 开头、与 `mygo.json` 版本一致的标签即可触发发布。三个平台的测试与打包全部通过后，[GitHub Releases](https://github.com/chen-985211/skiff/releases) 会提供 macOS Apple Silicon 的 DMG、Windows x64 的 EXE，以及 Linux x64 的 DEB、RPM 和 tar.gz。

## 许可证

Skiff 采用 [MIT 许可证](LICENSE)。
