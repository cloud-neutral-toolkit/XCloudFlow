# Skills 支持（外部 skills 读取）

目标：让 XCloudFlow/StackFlow Runner 能读取并使用外部 skills（标准流程/Runbook），并在 Cloud Run 无状态模式下可缓存。

## 1. Skills 的定位

- skills 是“规范与流程”，不是执行引擎
- skills 不应包含 secrets
- plugins 才负责真正调用云 API/执行 apply

## 2. Skills Sources

建议支持三类来源：

- `local`：本地目录（例如 `$CODEX_HOME/skills` 或某个 repo 的 `skills/`）
- `git`：git 仓库（按 ref/commit 拉取）
- `http`：只读 HTTP 拉取（用于发布版 skills）

建议配置（示例，未来可放在 `.xcloudflow.yaml`）：

```yaml
skills:
  sources:
    - name: codex-home
      type: local
      uri: /Users/shenlan/.codex/skills
    - name: cnt-control
      type: git
      uri: https://github.com/cloud-neutral-toolkit/github-org-cloud-neutral-toolkit
      ref: main
      path: skills
```

## 3. 加载与覆盖规则

- 以 `skills/<name>/SKILL.md` 为单元
- 同名 skill 的覆盖：后加载的 source 覆盖前者（必须记录来源与版本）
- runner 输出 summary 时可以引用 skill 路径，提示操作人员遵循对应 Runbook

## 4. 缓存（Cloud Run 友好）

- 外部 git/http 拉取结果建议缓存到 PostgreSQL：
  - `xcf.skill_sources`
  - `xcf.skill_docs`

这样 XCloudFlow 无状态扩容时也能复用 skills 内容与版本信息。

## 5. 安全

- 禁止把 skills 内容当作 secrets 渠道
- 禁止把私有 token 写入 skills
- 对 `git` source 的拉取凭证应使用 Secret Manager 或 CI 注入（短期）
