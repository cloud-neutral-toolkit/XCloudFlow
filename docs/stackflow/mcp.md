# StackFlow / XCloudFlow MCP 设计

目标：

- 让 Agent/ChatOps 通过 MCP 安全地调用 StackFlow/XCloudFlow 的 `validate/plan`，并以受控方式发起 `apply`。
- 让 XCloudFlow 既能 **对接外部 MCP servers**（例如某个 Cloud Run 服务暴露的 MCP 接口），也能 **自身作为 MCP server** 为外部系统提供可观测与编排信息。

## 1. XCloudFlow 的两种 MCP 角色

### 1.1 作为 MCP Server（对外提供信息与能力）

命令（规划）：

- `xcloudflow mcp serve`

对外暴露 tools（建议命名稳定、可脚本化）：

- `stackflow.validate`
- `stackflow.plan.dns`
- `stackflow.plan.iac`
- `stackflow.plan.deploy`
- `stackflow.plan.observe`
- `stackflow.runs.list` / `stackflow.runs.get`
- `stackflow.targets.list`
- `stackflow.skills.list` / `stackflow.skills.get`
- `mcp.servers.list` / `mcp.servers.get`

`apply` 系列 tools 默认应当：

- 在本地模式下禁用，除非显式开启
- 在 CI/Cloud Run 模式下必须满足环境门禁（见下文）

### 1.2 作为 MCP Client（对接外部系统）

XCloudFlow 可以注册外部 MCP servers（例如 Cloud Run 上的内部服务提供一些观测/配置接口），并把这些工具聚合到自身：

- 发现工具列表（并缓存）
- 以统一的审计/权限边界触发工具

建议将外部 MCP server 注册信息写入 PostgreSQL（XCloudFlow 无状态）：

- 表：`xcf.mcp_servers`、`xcf.mcp_tools_cache`
- Schema：见 `sql/schema.sql`

## 2. Cloud Run MCP 对接（示例模式）

典型模式：

- 某个 Cloud Run 服务暴露一个 MCP endpoint（HTTP），提供只读状态或受控写入能力。
- XCloudFlow 以 **服务到服务身份** 调用它（优先 OIDC，而非长期 token）。

建议的认证方式：

- `auth_type=oidc`
  - XCloudFlow 运行在 Cloud Run
  - 调用其他 Cloud Run 服务时使用 Google 的服务身份签发的 ID Token（audience=对方服务 URL）

在 `xcf.mcp_servers` 中保存：

- `base_url`：服务 MCP endpoint
- `auth_type`：`none|bearer|oidc`
- `audience`：OIDC audience（通常为对方服务 URL）

## 3. Apply 的门禁（强制）

MCP 能力不应绕过 GitHub Environments。
推荐的生产门禁：

- Plan 永远可跑（只读，无 secrets）
- Apply 只能在 CI 的受控环境中跑：
  - GitHub Actions job 绑定 `environment: prod`
  - required reviewers
  - secrets/OIDC 权限仅在 apply job 可用

XCloudFlow 作为 MCP server 时，也应在服务端再次校验：

- 禁止在无门禁上下文执行 apply
- apply 需要带 `gate` 参数或 CI 颁发的短期凭证

## 4. 审计与可观测

要求：

- 每次 MCP 调用都写审计事件（who/when/phase/config/env/inputs/result links）
- 审计写入 PostgreSQL（便于 Cloud Run 无状态横向扩展）

建议表：

- `xcf.runs`：一次 plan/apply 的记录
- `xcf.agent_events`：Agent/MCP 的事件流

Schema：见 `sql/schema.sql`
