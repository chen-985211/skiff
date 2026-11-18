# Skiff 开发入口

应用显示名为「轻舟」，仓库、模块、可执行程序、配置目录与环境变量统一使用 `skiff` / `Skiff` / `SKIFF_`，禁止使用中文名拼音。

## 架构

遵循 [架构约定](docs/engineering/architecture.md)：DDD + Clean Architecture，以限界上下文为第一目录维度。业务动作只进入 application；端口归 application；domain 不依赖 GUI、进程、网络或存储；presentation 与 infrastructure 不相互依赖。Platform 只负责装配和生命周期。

## 测试

- 所有生产行为采用 TDD：先运行能表达目标行为的失败测试，再实现，再重构。
- 阅读 [策略](docs/testing/TESTING_STRATEGY.md)，按 [矩阵](docs/testing/TESTING_MATRIX.md) 选择最低有效测试层。
- 按 [目录约定](docs/testing/TESTING_LAYOUT.md) 放置测试，按 [最低要求](docs/testing/TESTING_REQUIREMENTS.md) 验证；缺陷修复保留回归测试。
- 执行命令见 [运行手册](docs/testing/TESTING_RUNBOOK.md)。最终汇报新增、已运行、未运行的测试和剩余风险。

## 开发范围

先完成 HTTP/HTTPS 下载主流程。不要将 BT、浏览器接管、云同步或插件系统混入基础版。移除下载任务只清理任务，不删除用户文件。测试必须使用独立临时目录、本地 HTTP 服务和独立 aria2 进程。

## 界面与动效

修改或审查 MyGo 界面时使用项目技能 [液态玻璃界面](.agents/skills/liquid-glass-interface/SKILL.md)，通过 `make check-ui` 验证配色与动效契约。
