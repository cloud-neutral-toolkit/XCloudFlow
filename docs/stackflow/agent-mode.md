# Agent 模式（XCloudFlow）

目标：让 XCloudFlow 以 Agent 方式常驻运行（例如 Cloud Run 或 VM/K8s），持续执行 StackFlow 的 plan/apply、聚合外部 MCP、并维护可审计的状态与“记忆”。

## 1. Agent 能力范围

- 定时或事件驱动运行：
  - `validate`
  - `plan dns/iac/deploy/observe`
  - （受门禁）`apply dns/iac/deploy/observe`
- 维护运行记录与事件流
- 缓存外部 MCP tools 列表
- 缓存外部 skills

## 2. 状态与记忆存储

约束：XCloudFlow **自身无状态**。

因此需要把以下数据写入 PostgreSQL（推荐 `postgresql.svc.plus`）：

- runs（每次 plan/apply 记录）
- agent events（事件/日志流）
- leases（分布式锁，避免多实例重复 apply）
- mcp servers registry + tools cache
- skill sources + docs cache

对应 schema：`sql/schema.sql`

## 3. 多实例并发与锁

在 Cloud Run 多实例下，必须避免重复执行同一 stack/env/phase：

- 通过 `xcf.leases` 获取租约（带过期时间）
- 仅持有 lease 的实例执行 apply
- plan 可并发，但建议限流并写审计

## 4. Cloud Run 部署建议

- 服务无状态：所有状态/缓存写 PostgreSQL
- 使用 Cloud Run Secret Manager 注入数据库连接串（或使用 IAM DB Auth）
- 调用其他 Cloud Run 服务（MCP client）优先用 OIDC ID Token
- 日志输出到 stdout/stderr，由 Cloud Logging 收集

## 5. 迁移与初始化

- 提供一个 `migrate` 子命令（规划）：`xcloudflow db migrate --dsn ... --file sql/schema.sql`
- 或由 CI/运维流程在上线前执行 schema 初始化
