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
