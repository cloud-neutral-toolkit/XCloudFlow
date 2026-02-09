# StackFlow MCP 设计

目标：让 Agent/ChatOps 通过 MCP 安全地调用 StackFlow 的 validate/plan，并以受控方式发起 apply。

## 1. MCP Server

- 命令：`stackflow mcp serve`
- 运行模式：
  - 本地开发：直接读本地文件系统与 git
  - CI 环境：只读执行（plan/validate），apply 需环境门禁

## 2. Tools（建议）

- `stackflow.validate`
- `stackflow.plan.dns`
- `stackflow.plan.iac`
- `stackflow.plan.deploy`
- `stackflow.plan.observe`
- `stackflow.apply.*`（未来；需要 gate 参数与审计）

## 3. 安全模型

- MCP 默认只读：不携带云凭证、不允许 apply
- apply 只能在：
  - GitHub Actions 的受控环境
  - 或本地显式允许（例如 `--dangerously-allow-apply`）

## 4. 审计

- 每次 MCP 调用记录：who/when/config/env/phase
- 输出 artifacts 只包含非敏感信息
