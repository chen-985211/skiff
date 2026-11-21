# Skiff 开发入口

应用显示名为 Skiff，仓库、模块、可执行程序、配置目录与环境变量统一使用 `skiff` / `Skiff` / `SKIFF_`，禁止使用中文名拼音。

## 架构

遵循 [架构约定](docs/engineering/architecture.md)：DDD + Clean Architecture，以限界上下文为第一目录维度。业务动作只进入 application；端口归 application；domain 不依赖 GUI、进程、网络或存储；presentation 与 infrastructure 不相互依赖。Platform 只负责装配和生命周期。

## 测试

- 所有生产行为采用 TDD：先运行能表达目标行为的失败测试，再实现，再重构。
- 阅读 [策略](docs/testing/TESTING_STRATEGY.md)，按 [矩阵](docs/testing/TESTING_MATRIX.md) 选择最低有效测试层。
- 按 [目录约定](docs/testing/TESTING_LAYOUT.md) 放置测试，按 [最低要求](docs/testing/TESTING_REQUIREMENTS.md) 验证；缺陷修复保留回归测试。
- 执行命令见 [运行手册](docs/testing/TESTING_RUNBOOK.md)。最终汇报新增、已运行、未运行的测试和剩余风险。

每次完成修改并通过相应验证后，将本次改动提交到本地 Git。

每轮修改交付时，用 `make dev` 启动当前分支的最新开发版。确认主窗口已显示、下载引擎已就绪后，保留开发版运行，供用户端到端验收。

## 开发范围

先完成 HTTP/HTTPS 下载主流程。不要将 BT、浏览器接管、云同步或插件系统混入基础版。移除下载任务只清理任务，不删除用户文件。测试必须使用独立临时目录、本地 HTTP 服务和独立 aria2 进程。

## 界面与动效

修改或审查 MyGo 界面时使用项目技能 [液态玻璃界面](.agents/skills/liquid-glass-interface/SKILL.md)，通过 `make check-ui` 验证配色与动效契约。

控件创建时绑定 [motion.go](internal/contexts/transfer/presentation/motion.go) 的动效。条件显示的面板和状态文字要有进入与退出过渡。可打断的展开或选择使用稳定元素上的 `AnimateTo` 驱动高度、位置或透明度。为新交互覆盖中间帧和减少动态效果模式。UI 门禁检查控件自身的创建链，报错带文件与行号；CI 在推送和 PR 时运行同一组契约及渲染测试。

所有平台的控件统一使用连续曲率圆角。圆角尺寸集中在 [corners.go](internal/contexts/transfer/presentation/corners.go)，同一表面的填充、边框与裁剪共用一个尺寸。Logo 的圆角用连续曲线路径绘制，修改后运行 `swift scripts/make-icon.swift` 同步应用图标。`make check-ui` 同时检查圆角开关、SVG 资源和实际渲染轮廓。

界面中的下拉选择统一使用 [dropdown 组件](internal/contexts/transfer/presentation/dropdown.go)。键盘导航、选中反馈、弹层外观和开合曲线由组件负责。新增入口通过 `make check-ui` 检查。
