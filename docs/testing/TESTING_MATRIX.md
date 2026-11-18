# 测试选择矩阵

| 修改 | 最低验证 |
| --- | --- |
| 链接、状态、进度与动作资格 | unit 参数矩阵 |
| 添加/暂停/继续/重试/移除/恢复用例 | unit：Engine/Repository 端口替身 |
| JSON-RPC 参数、鉴权与错误 | integration：本地 HTTP RPC 服务 |
| 文件仓储与版本 | integration：真实临时文件、损坏与往返 |
| aria2 生命周期与恢复 | integration：真实 aria2 + 本地下载服务 |
| UI 输入、筛选、空/错误状态 | MyGo 无窗口组件测试；原生运行视觉检查 |
| 分层与依赖 | contract：解析实际 Go import |
| 打包与启动 | build + 本机产物启动检查 |
| 文档与格式 | 相关链接、命令与格式检查 |
