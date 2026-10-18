# TarsLiteLLM 变更日志

## v1.1.0 · 集成版本 — 请求重试与指数退避
- **新增**：HTTP 请求自动重试机制（最多 3 次）
- **新增**：指数退避 + 抖动策略，避免雪崩效应
- **优化**：4xx 客户端错误不触发重试，减少无效请求
- **测试**：新增 `TestRetry_SuccessAfterTransientFailure`、`TestRetry_NoRetryOn4xx`、`TestCalculateBackoff`
