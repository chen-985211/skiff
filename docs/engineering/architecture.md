# Skiff 架构

参考 `/Users/nature/Development/cleancode/docs/engineering/architecture.md` 的 DDD + Clean Architecture、上下文优先组织、应用层端口、单一规则 owner 与外层装配。

基础版只需要 Transfer（下载任务）一个限界上下文，不预建空的共享内核或无业务含义的框架。

```text
cmd/skiff/                          程序入口
internal/contexts/transfer/
  domain/                           链接、任务聚合、状态与动作规则
  application/                      下载用例、端口、只读 DTO
  infrastructure/aria2/             JSON-RPC、受管子进程
  infrastructure/persistence/       原子文件仓储
  presentation/                     MyGo 原生界面、临时交互状态
internal/platform/desktop/          配置路径、系统能力、装配、退出清理
tests/{unit,integration,contract}/  按验证层组织，不镜像生产分层
```

依赖向内：presentation → application → domain；infrastructure → application/domain。上下文不得 import platform。表现层不读写文件、不管理进程、不直接调用 aria2。架构契约测试检查 Go import，Go 编译器检查循环依赖。

Task 聚合拥有稳定任务 ID、下载链接、目标目录、状态和动作资格。Transfer Service 是唯一业务入口，串行处理操作与刷新，避免旧进度覆盖新的暂停/移除结果。application 定义 Engine、Repository、Desktop 端口；aria2 和文件仓储负责实现。

持久化任务记录是任务目录与已提交意图的来源；aria2 的实时观测是运行进度与完成状态的来源。aria2 session 仅是引擎恢复资料；重启时由恢复用例对账，未完成任务以暂停状态恢复。UI 只持有 DTO 副本。

首版验收：批量添加有效 HTTP/HTTPS 链接、选目录、进度/速度、暂停、继续、失败重试、移除但保留文件、重启恢复、显示错误。网络、存储和进程操作不阻塞 UI 线程。

续传错误由 aria2 适配器根据错误码映射为稳定的 `resume_unsupported`，不通过英文错误文案猜测。Task 聚合拥有该失败类型的动作资格：不重复普通重试，只允许显式从头下载。其他失败（包括旧版本缺少分类的记录）保留普通重试，同时允许安全的显式从头下载。新下载目标由应用用例提交到仓储，原部分文件保持不变；OutputName 随任务持久化并在恢复时传给引擎。表现层只负责解释和确认。
