<p align="center">
  <img src="resources/icon.png" width="112" height="112" alt="Skiff Logo">
</p>

<h1 align="center">Skiff</h1>

<p align="center">桌面下载工具，支持 HTTP、HTTPS、FTP、磁力链接和 BitTorrent 种子。</p>

<p align="center">
  <a href="https://github.com/chen-985211/skiff/releases"><img src="https://img.shields.io/badge/platform-macOS%20%C2%B7%20Windows%20%C2%B7%20Linux-496C99?labelColor=374357" alt="平台：macOS、Windows、Linux"></a>
  <a href="https://github.com/aria2/aria2"><img src="https://img.shields.io/badge/engine-aria2-32766F?labelColor=374357" alt="下载引擎：aria2"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-796194?labelColor=374357" alt="MIT 许可证"></a>
</p>

<div align="center">

https://github.com/user-attachments/assets/8fa22cc3-e91a-4537-87f4-756609c0314c

</div>

## 文件下载

- 批量粘贴文件链接，自动跳过重复项。也可以直接粘贴浏览器复制的 cURL，导入下载地址和请求头。
- 自动分段下载，可为单个文件设置连接数上限。支持断点续传，也可添加备用地址，在主地址失败时自动切换。
- 自选保存目录、切换最近用过的目录或修改文件名。遇到同名文件时保留两份。
- 支持 FTP 账号密码、自定义 HTTP 请求头和单任务代理。

## BitTorrent 与磁力

- 打开种子文件或粘贴磁力链接，在文件树中勾选要下载的文件。磁力任务也可设置等待一段时间后自动下载全部文件。
- 管理 Tracker 订阅，使用 DHT 和局域网发现节点。任务详情中可查看节点、Tracker 和分片进度。
- 下载完成后继续做种，达到指定分享率或时长时自动停止。已有文件可校验、移动或重新定位后继续做种。
- 从文件或文件夹创建 BitTorrent v1 种子，直接使用原文件做种。

## 任务管理与设置

- 按文件名或链接搜索，按状态筛选。可批量暂停、继续，也可重试失败任务。移除任务会保留已下载的文件。
- 调整同时下载的任务数，有空位时优先下载指定任务。可随时调整全局限速或单个任务的限速。
- 配置代理、DNS、磁盘缓存、最小分段大小和文件预分配。
- 支持浅色、深色和跟随系统的主题，可切换背景流光。下拉菜单支持键盘操作，并跟随系统的「减少动态效果」设置。
