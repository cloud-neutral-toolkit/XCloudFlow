# StackFlow Exec Plugin 规范（v1）

本页定义 exec plugin 的发现、调用与输入输出协议。

## 1. 命名与发现

插件二进制命名：

- `stackflow-plugin-<domain>-<name>`

示例：

- `stackflow-plugin-dns-cloudflare`
- `stackflow-plugin-iac-terraform`
- `stackflow-plugin-deploy-ansible`
- `stackflow-plugin-observe-prometheus`

runner 搜索路径：

1. `./plugins/`
2. `$PATH`

## 2. 子命令

插件必须支持：

- `info`：输出插件元信息
- `plan`：输入 StackFlow Config/上下文，输出该插件可生成的 plan（可选）
- `apply`：输入 plan + credentials context（由 runner 注入），执行并输出结果

## 3. stdin/stdout 协议

- stdin：JSON
- stdout：JSON（成功）
- stderr：人类可读日志
- exit code：0 成功，非 0 失败

建议的通用 envelope：

```json
{
  "apiVersion": "stackflow.plugin/v1",
  "kind": "Request",
  "phase": "dns-apply",
  "env": "prod",
  "stack": { ...normalized StackFlow config... },
  "plan": { ...phase plan... }
}
```

返回：

```json
{
  "apiVersion": "stackflow.plugin/v1",
  "kind": "Response",
  "ok": true,
  "changes": [],
  "artifacts": [],
  "links": []
}
```

## 4. Credentials 注入

- runner 通过环境变量注入（不落盘）：
  - `CLOUDFLARE_API_TOKEN`
  - `ALIYUN_AK`, `ALIYUN_SK`
  - `GOOGLE_OIDC_TOKEN`（或通过官方 auth action 注入）

插件不得把 secrets 写入输出或 artifacts。

## 5. 幂等与安全

- apply 必须尽可能幂等：重复执行不应产生重复资源或破坏性变更
- 对破坏性操作必须显式声明（例如 `dangerous: true`），runner/CI 在 prod 需额外审批
