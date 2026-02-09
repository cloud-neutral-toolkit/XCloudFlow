# StackFlow 配置规范（v1alpha1）

本页描述 `apiVersion: gitops.svc.plus/v1alpha1`、`kind: StackFlow` 的字段约定。

> 当前 gitops runner（Python 版）仅实现了 validate + dns-plan；此规范同时覆盖未来 iac/deploy/observe phases。

## 1. 顶层结构

```yaml
apiVersion: gitops.svc.plus/v1alpha1
kind: StackFlow
metadata:
  name: <stack-id>

global:
  domain: <root-domain>
  dns_provider: <cloudflare|alicloud|...>
  cloud: <gcp|aws|...>
  gcp_project: <optional>
  environments: { <env>: { ...overrides... } }
  gitops: <url>
  playbooks: <url>
  iac_modules: <url>

targets:
  - id: <string>
    type: <vercel|vhost|cloud-run|...>
    domains: [<fqdn>...]
    dns: { records: [...] }
```

## 2. environments 覆盖规则

- runner 接收 `--env dev`
- 对 `global` 做浅合并覆盖：`global.environments.dev.*` 覆盖 `global.*`

> 不对 targets 做自动覆盖；targets 的 env 覆盖（如 domains）可在后续版本明确规范。

## 3. DNS Records

单条 record：

- 必填：`name`, `type`
- 二选一：`value` 或 `valueFrom`
- 可选：`ttl`、`proxied`

`valueFrom` 用于引用 `endpoints.*` 这类由 iac-apply 回填的字段。

## 4. 约束（validate 最少要做）

- `targets[].domains[]` 必须在 `global.domain` 之下
- `dns.records[]` 必须包含 `name` + `type` + (`value` or `valueFrom`)
- `type` 统一大写（A/AAAA/CNAME/TXT...）
- 去重建议：同一 `(fqdn,type)` 只能出现一次

## 5. 建议的演进（兼容性）

- v1alpha1 保持字段向后兼容：新增字段只增不删
- runner 归一化：输出中回填标准字段（例如统一 `mem_mib`/`memMiB`）
- 后续可引入 JSONSchema 并在 CI 中作为 validate 的第一步
