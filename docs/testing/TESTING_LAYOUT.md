# 测试目录

`tests/unit/transfer` 放聚合、用例和 view model 行为；`tests/unit/platform` 放平台诊断结构和输出行为；`tests/integration/transfer` 放 RPC、存储、真实下载生命周期；`tests/contract` 放依赖方向。文件使用 `*_test.go`，测试包通过公开契约访问生产模块。共享夹具只在确有重复时提取，禁止从生产代码 import 测试。

integration 使用 `integration` build tag；默认 `go test ./...` 不启动系统进程。任何截图、测试日志和临时产物放在 git 忽略的 `test-results/` 或临时目录，真实任务目录不参与测试。
