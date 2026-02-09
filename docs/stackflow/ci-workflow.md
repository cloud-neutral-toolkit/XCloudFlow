# GitHub Actions 编排（StackFlow plan/apply）

本页描述控制仓库中 `stackflow.yaml` 如何演进为 plan/apply 两类 workflow，并如何使用 GitHub Environments 控制权限。

## 1. 工作流拆分

- `stackflow.plan.yaml`
  - 仅 validate + plan
  - 权限：`contents: read`
  - secrets：通常不需要（可选 `GITOPS_CHECKOUT_TOKEN` 只读）

- `stackflow.apply.yaml`
  - dns-apply / iac-apply / deploy / observe
  - 使用 `environment: prod|dev`
  - secrets/OIDC 仅在对应 job 可用

> 也可以保留一个 workflow 文件，但分 job 并用 `if:` 条件和 `environment:` 做门禁。

## 2. Secrets / OIDC 规划

- Plan/Validate：
  - `GITOPS_CHECKOUT_TOKEN`（可选，只读）

- Apply：
  - `CLOUDFLARE_API_TOKEN`
  - `ALIYUN_AK` / `ALIYUN_SK`
  - GCP：推荐 OIDC（Workload Identity Federation），避免长期 JSON key
  - `VERCEL_TOKEN`（可选）

## 3. Sources 驱动 checkout

StackFlow YAML 中的：

- `global.gitops`
- `global.playbooks`
- `global.iac_modules`

不应仅作为注释；应成为 workflow 的 checkout 输入：

- plan job：至少 checkout `gitops`（取 StackFlow YAML）
- iac/deploy/observe job：按需 checkout `playbooks`、`iac_modules`

## 4. 建议的 job 顺序

- `validate`
- `plan_dns`
- `plan_iac`
- `plan_deploy`
- `plan_observe`
- `apply_dns` (env gate)
- `apply_iac` (env gate)
- `apply_deploy` (env gate)
- `apply_observe` (env gate)

## 5. Artifacts 与 Summary

- 每个 phase 输出 JSON artifact
- 额外生成 `stackflow.summary.md` 写入 `$GITHUB_STEP_SUMMARY`
- apply 阶段额外输出 links（云控制台、terraform plan、ansible logs、monitor dashboards）
