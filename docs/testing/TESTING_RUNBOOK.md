# 测试运行手册

依赖：Go 1.27.3+、aria2（macOS 可用 `brew install aria2`）；MyGo 版本锁定在 go.mod。

| 目的 | 命令 |
| --- | --- |
| 获取依赖 | `go mod download` |
| 格式化 | `gofmt -w cmd internal tests` |
| unit + 架构契约 | `go test ./...` |
| Go 文件 800 行上限 | `go test -count=1 ./tests/contract -run '^TestGoFileLineLimit$'` |
| 国际化契约 | `make check-i18n` |
| 日志与错误契约 | `make check-diagnostics` |
| 本地 UI 门禁（设计规则与界面回归） | `make check-ui` |
| 目标 Red/Green | `go test ./tests/unit/transfer -run TestName -v` |
| 集成 | `go test -tags integration ./tests/integration/... -v` |
| 日常完整检查（格式、vet、全部用例、构建） | `make check` |
| 深度竞态检查（包含软件渲染） | `make test-race` |
| 静态检查 | `go vet ./...` |
| 编译 | `go build ./cmd/skiff` |
| 本机热重载（macOS 开发包使用 ad hoc 签名） | `make dev` |
| 单次开发运行 | `go run ./cmd/skiff` |
| 原生打包 | `go tool mygo build` |

像素回归使用 `TestRender` 前缀。`make check` 用常规构建运行这组用例，用竞态检测运行其余单元、契约和集成用例。单独复现某个测试时可加 `-count=1` 强制重新执行。

真实下载测试只访问本机 HTTP 服务；不需要密钥或账户。若找不到 aria2，integration 应失败并解释依赖，不能静默跳过。基础版仅宣告当前实测的 macOS arm64，其他平台需原生验证。

## 局部操作闪烁排查

全局限速和任务限速曾在选中一项时调用 `View.run`。这个方法会在后台操作期间设置窗口级 `View.Busy`。设置页的目录与网络等控件因此变灰，任务卡片的操作行会消失，底部「新建下载」也会变灰。保存完成后的下一次绘制又把它们恢复。引擎应答和写盘耗时决定中间状态是否会被画出来，单看展开后的菜单或保存后的截图容易漏掉。此前主题和背景流光的保存也触发过同类问题，见提交 `48dc419`。

1. 分开操作一次展开、移动高亮、取消，再打开并选中一个新值。仅在提交后出现时，沿保存回调查找读取同一个忙碌状态的其他控件。展开或取消就出现时，单独检查弹层遮罩、主题作用域和背景绘制。
2. 在组件测试中冻结背景流光，保留控件动效，把后台完成回调留到测试指定的帧。比较菜单及其阴影之外的固定区域，同时检查该处的按钮是否仍然存在并可用。覆盖选择当帧、保存中和回调后的画面。
3. 在 macOS 开发版复跑，用临时任务目录取证。若本机保存快到难以观察，可在测试夹具中延迟写入，再用相同操作录制修复前后画面。涉及背景流光时也用动态背景走一遍。

复跑 `go test -count=1 ./tests/unit/transfer -run 'TestRenderChangingDownloadLimitKeepsUnrelatedControlsStable|TestRateSelectionsKeepLatestValueAndRollbackInPlace'`。两个用例已包含在 `make check-ui` 中。

如果只有菜单内的高亮在换行时断开，先在两行之间快速往返，再用方向键反向。旧实现给每行单独淡入淡出，中途两行的高亮都会变浅。共享底板应经过中间位置，反向时从当前位置接续。系统开启「减少动态效果」时应直接定位。在全局限速和任务限速两个入口各从「不限速」切至「512 KiB/s」，检查按钮边界是否稳定。

复跑 `go test -count=1 ./tests/unit/transfer -run 'TestRenderDropdownHighlightFollowsAndReversesBetweenRows|TestRatePresetSelectionKeepsTriggerInPlace'`。
