# TarsLiteLLM 变更日志

## v1.1.0 · 集成版本 — 请求重试与指数退避
- **新增**：HTTP 请求自动重试机制（最多 3 次）
- **新增**：指数退避 + 抖动策略，避免雪崩效应
- **优化**：4xx 客户端错误不触发重试，减少无效请求
- **测试**：新增 `TestRetry_SuccessAfterTransientFailure`、`TestRetry_NoRetryOn4xx`、`TestCalculateBackoff`

## v1.2.0 · 集成版本 — 用量追踪与指标暴露
- **新增**：`UsageTracker` 结构体，原子计数请求/成功/失败/总延迟
- **新增**：健康检查 `/health` 端点暴露实时用量快照
- **优化**：每个请求自动记录延迟与结果，无需手动埋点
- **测试**：新增 `TestUsageTracker_RecordAndSnapshot`、`TestUsageTracker_Concurrent`、`TestHealthHandler_IncludesUsage`

## v1.3.0 · 集成版本 — 配置热重载
- **新增**：`Client.ReloadConfig()` 方法，从环境变量重新读取配置
- **新增**：SIGHUP 信号触发配置热重载，无需重启进程
- **优化**：`Client.mu` 读写锁保护并发配置更新与请求执行
- **测试**：新增 `TestReloadConfig`、`TestReloadConfig_Partial`、`TestClientReq_ConcurrentWithReload`

## v1.4.0 · 集成版本 — 分级日志
- **新增**：`Logger` 结构体，支持 INFO / WARN / ERROR 三级日志
- **新增**：`TARS_LOG_LEVEL` 环境变量控制日志级别
- **优化**：所有内部日志统一走 Logger，重试日志降级为 WARN
- **测试**：新增 `TestLogger_Levels`、`TestLogger_InfoSuppressedWhenError`、`TestClient_HasLogger`

## v1.5.0 · 集成版本 — 连接池与优雅关闭
- **新增**：`newPooledHTTPClient()` 返回调优连接池（MaxIdleConns=100 / MaxConnsPerHost=50 / IdleConnTimeout=90s）
- **新增**：SIGINT/SIGTERM 触发 `srv.Shutdown()`，10s 优雅关闭超时
- **优化**：Keep-Alive 默认开启，减少连接重建开销
- **测试**：新增 `TestNewClient_HasPooledTransport`、`TestGracefulShutdown`
