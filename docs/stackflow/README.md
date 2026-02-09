# StackFlow Docs (Go Runner)

本目录描述 Cloud-Neutral Toolkit 的 StackFlow 体系：

- 声明源（Source of Truth）：`cloud-neutral-toolkit/gitops/StackFlow/*.yaml`
- 控制平面编排（Control Plane）：`cloud-neutral-toolkit/github-org-cloud-neutral-toolkit/.github/workflows/stackflow*.yaml`
- 执行与规划引擎（Go Runner）：`stackflow` CLI + MCP + plugins + skills

目标：用一份声明把 `DNS -> IAC -> Deploy -> Observe` 的完整链路打通，并通过 **Plan/Apply 分离** + **GitHub Environments** 实现生产级门禁。

## 文档索引

- 设计总览：`runner-design.md`
- Phase 与产物：`phases.md`
- 配置规范：`config-spec.md`
- 插件规范：`plugin-spec.md`
- MCP 规范：`mcp.md`
- GitHub Actions 编排：`ci-workflow.md`

## 与 XCloudFlow 的关系

XCloudFlow 是通用的多云控制面与编排框架；StackFlow 是面向 Cloud-Neutral Toolkit 业务栈的轻量 DSL 与 runner。

- StackFlow 可以视为 XCloudFlow 的一种 DSL/Composition（更贴近“业务栈”而非“资源组合”）
- StackFlow Runner 可以作为 XCloudFlow 的一个 Engine/Orchestrator 适配层

详见：`runner-design.md` 的“对齐 XCloudFlow”章节。
