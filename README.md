<p align="center">
  <img src="resources/icon.png" width="112" height="112" alt="Skiff logo">
</p>

<h1 align="center">Skiff</h1>

<p align="center">A desktop download manager built with MyGo, a native Go UI, and GPU rendering. Skiff handles direct file links, magnet links, and torrent files.</p>

<p align="center">
  <strong>English</strong> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.zh-Hant.md">繁體中文</a> · <a href="README.ja.md">日本語</a>
</p>

<p align="center">
  <a href="https://github.com/chen-985211/skiff/releases/latest"><img src="https://img.shields.io/github/v/release/chen-985211/skiff?label=version&amp;color=blue" alt="Latest Skiff release"></a>
  <a href="https://github.com/chen-985211/skiff/releases"><img src="https://img.shields.io/badge/platform-macOS%20%C2%B7%20Windows%20%C2%B7%20Linux-lightgrey" alt="macOS, Windows, and Linux"></a>
  <a href="https://github.com/aria2/aria2"><img src="https://img.shields.io/badge/engine-aria2-orange" alt="aria2 download engine"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-brightgreen" alt="MIT License"></a>
</p>

<div align="center">

https://github.com/user-attachments/assets/52e5421b-7dac-4763-a994-52b8a602be83

</div>

## Download features

### Direct file downloads

- **Add links.** Under **New Download → Link**, paste HTTP, HTTPS, or FTP file URLs, or a cURL command copied from your browser. Enter one link per line, up to 100 per batch. Skiff queues valid links, skips duplicates, and leaves invalid or excess lines in the input.
- **Save and resume.** Use the default folder, choose a recent folder, or rename a single file. HTTP downloads follow redirects and recognize server-provided filenames. Paused downloads resume from their checkpoint, including when the server provides no version identifier. If the remote file changed or the server refuses to resume, start over in a new file and retain the partial download.
- **Queue and connections.** Skiff downloads up to 10 tasks at once by default. Set the limit from 1 to 200, or choose **Download next** for a waiting task. Skiff normally picks one to four connections based on file size and server support for range requests. In **Advanced options**, set a limit from 1 to 16. The task details show the actual connection count.
- **Speed and network.** Set a shared limit under **Settings → Global speed limit** or a per-task limit in the task details. Both offer presets and custom limits that take effect during downloads. **Proxy and DNS** covers system proxy settings, direct connections, authenticated HTTP proxies, bypass lists, and DNS modes.
- **Advanced options.** Set mirror URLs, request headers, or a proxy for a task. If the primary URL fails, Skiff tries mirrors in order and reuses progress once it confirms the content matches. **Settings → Performance and Disk** controls the disk cache, minimum segment size, and file allocation.

### BitTorrent and magnet links

- **Choose files to download.** Under **New Download → BT**, open a local `.torrent` file or paste a magnet link and choose **Fetch Files**. Select files in the tree and choose **Start**. For a magnet link, you can schedule all files to download after 30, 60, or 120 seconds. Changing the selection cancels the countdown.
- **Manage peers.** Under **Settings → BitTorrent Network**, configure tracker subscriptions, DHT, local peer discovery, the listening port, and peer limits. Task details show peers, trackers, and piece progress.
- **Seed and reuse files.** Keep seeding after a download completes and stop at a share ratio or time limit. Use the task details to verify, relocate, or move complete files and resume seeding.
- **Create torrents.** Under **BitTorrent options → Create Torrent…**, choose a local file or folder to create a `.torrent` file. Select **Seed the original files** to start sharing when you save it.

### Tasks and controls

- Task cards show progress, speed, and failure details. Select a filename to see its source and local path. Select the folder in the lower-left corner to open the download folder.
- Pause, resume, or retry tasks. Filter by status, name, or link to pause or resume tasks in bulk. Unfinished downloads return paused after a restart. Removing a task keeps the files on disk.
- Press `⌘N` or `Ctrl+N` to create a download and `⌘F` or `Ctrl+F` to search. Reopen the download form in the same session to continue editing its links and destination.
- Speed limits, BT options, and settings share the same dropdown. Use ↑ or ↓ to move the highlight, Enter to confirm, and Esc to close. The highlight follows quick changes and moves instantly with the system's reduced-motion preference. Adjust the theme and background glow under **Settings → Appearance**.
- Skiff supports Simplified Chinese, English, Traditional Chinese, and Japanese. Choose a language under **Settings → Appearance → Language**, or follow the system language.

## Getting started

### Open Skiff for the first time on macOS

1. On a Mac with Apple silicon, [download the macOS DMG][macos-dmg], open it, and drag **Skiff.app** into **Applications**.
2. Open **Skiff** from **Applications**. This build uses ad hoc signing and has no Apple notarization. If macOS shows a developer verification prompt, dismiss it.
3. Verify that the app came from the official release above. In **System Settings → Privacy & Security**, find Skiff, click **Open Anyway**, and confirm. You can then open Skiff normally. [Apple's guide](https://support.apple.com/en-us/102445)

**Alternatively, open Skiff from Terminal.** After copying the app from the official DMG to **Applications**, run this command to remove the quarantine attribute from that copy of Skiff and launch it.

```sh
xattr -dr com.apple.quarantine /Applications/Skiff.app && open /Applications/Skiff.app
```

### Start a download

Open Skiff and choose **New Download** at the bottom of the window. If you see **Get Engine**, select it to download and verify the appropriate aria2c build. The engine has x64 and ARM64 builds for macOS, Windows, and Linux. Initial engine setup requires access to GitHub. Skiff reads system proxy settings on macOS and Windows, and reads environment variables such as `HTTPS_PROXY`, `ALL_PROXY`, and `NO_PROXY` on Linux. [aria2c builds used by Skiff](https://github.com/motrixapp/aria2/releases/tag/v1.37.0-motrix.24)

[macos-dmg]: https://github.com/chen-985211/skiff/releases/latest

## Development

Install Go and run the following commands from the repository root. Go selects the toolchain specified in [go.mod](go.mod).

```sh
go mod download
make dev
```

`make dev` rebuilds and launches the development app after code changes. It uses a local ad hoc signature on macOS. Running `go tool mygo dev` directly lets MyGo choose a development certificate from your keychain. Press Ctrl+C to stop hot reload.

Skiff runs aria2 as a separate process. For integration tests on macOS, install the engine with `brew install aria2`, or point `SKIFF_ARIA2_BINARY` to an existing aria2c executable.

## Testing and building

```sh
make test                 # Unit and architecture contract tests
make test-integration     # Real aria2, local HTTP, RPC, and file storage
make check-ui             # UI contracts, interactions, and animation regressions
make check                # Formatting, vet, all tests (including race detection), and build
make package              # Native package for the current platform
```

The macOS app is written to `build/darwin-arm64/Skiff.app`. Local MyGo packages on macOS use ad hoc signing.

To publish a release, push a version tag prefixed with `v` that matches `mygo.json`. Once tests and packaging succeed across all three platforms, [GitHub Releases](https://github.com/chen-985211/skiff/releases) provides a DMG for Apple silicon Macs, an EXE for Windows x64, and DEB, RPM, and tar.gz packages for Linux x64.

## License

Skiff is available under the [MIT License](LICENSE).
