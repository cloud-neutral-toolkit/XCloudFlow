# Phases 与产物（Artifacts）

本页给出每个 phase 的输入/输出契约，方便 CI/MCP/plugins 对齐。

## 1. 通用输入

- `--config <path>`：StackFlow YAML
- `--env <name>`：使用 `global.environments.<name>` 覆盖 global（浅合并）
- `--format json`：默认 JSON 输出

## 2. validate

输出（示例字段）：

```json
{
  "ok": true,
  "stack": "svc-plus",
  "env": "prod",
  "domain": "svc.plus",
  "dns_provider": "cloudflare",
  "cloud": "gcp",
  "targets": 3
}
```

## 3. dns-plan

输出：扁平化 records（用于 dns-apply）

```json
{
  "stack": "svc-plus",
  "env": "prod",
  "global": {"domain":"svc.plus","dns_provider":"cloudflare"},
  "records": [
    {"target":"vercel-console","name":"www","type":"CNAME","value":"cname.vercel-dns.com.","proxied":false}
  ]
}
```

## 4. iac-plan

输出：module 调用清单（用于 terraform/pulumi）

建议结构：

- `modules[]`: {target, engine, source, inputs, outputs[]}
- `state`: {backend, workspace, lock}

## 5. deploy-plan

输出：部署触发清单（用于 dispatch/workflow_call/ansible）

建议结构：

- `actions[]`: {target, type, mode, repo, ref, payload, requires[]}

## 6. observe-plan

输出：监控资源清单（用于写入 observability repo 或直接 apply）

建议结构：

- `blackboxTargets[]`
- `scrapeJobs[]`
- `alertRules[]`

## 7. apply phases

apply 输出建议统一：

- `ok`: bool
- `changes[]`: {resource, action, before, after}
- `links[]`: 外部控制台/日志链接
- `artifacts[]`: 输出文件路径（state、inventory、diff、报告）
