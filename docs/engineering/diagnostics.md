# Skiff 日志与错误

Transfer 领域规则在 `domain/errors.go` 声明稳定错误码，用例和适配器在 `application/errors.go` 声明对外错误码。领域错误穿过用例时保留身份，包装外部失败时通过 `application.Wrap` 保留原始 cause。调用方使用 `errors.Is`、`errors.As` 或 `application.CodeOf`。下载任务继续使用已存储的失败分类，新记录的应用失败也携带错误码。

表现层在 `presentation/error_messages.go` 将错误码映射为 [国际化文案](i18n.md)。未知错误统一显示可重试提示。任务失败的修复操作由领域规则决定，断点续传、文件已变和 FTP 登录使用各自的文案。来自引擎、操作系统或服务端的原始错误只用于本地诊断。

平台层使用 `internal/platform/diagnostics` 将应用诊断写到标准错误流，每行一个 JSON 对象。记录包含 UTC `timestamp`、`level`、`scope`、`operation`、`outcome`、`durationMs`、`correlationId`。失败还包含 `error.code` 与 `error.isExpected`。预期校验失败使用 `warn`，非预期异常使用 `error`，下载引擎就绪使用 `info`。退出清理成功保持静默，高频轮询的连续失败仅记录首次。

日志字段使用稳定范围与操作标识。禁止记录错误原文、堆栈、完整链接、用户路径、认证信息、请求头、进程输出和文件内容。aria2 自己的 `engine.log` 属于引擎诊断文件。

`make check-diagnostics` 检查标准日志 API 的使用边界、领域与用例的错误构造、界面是否直接读取错误原文，以及错误码和词库的映射，并运行诊断输出测试。相同契约测试随 `make check` 和 CI 运行。
