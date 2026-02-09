# XCloudFlow 全栈设计

本文档将“生命周期 × 变化频率”的决策矩阵固化到 XcloudFlow 的整体架构、DSL、执行引擎、编排治理与落地路线中，目标是实现多云统一、分层治理、兼容多种自动化引擎，并以 GitOps 作为唯一真相源。

## 0. 决策矩阵（策略基线）

| 类别 | 典型资源 | 变化频率 | 首选工具 | 调谐策略 | 漂移策略 | 变更门禁 | 备份/RPO |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Foundation（地基） | VPC/子网/路由、NAT、KMS、基础 DNS zone、K8s 集群 | 低 | Terraform/Pulumi | 检测为主（不开自动纠偏） | Alert（禁止自动修） | 强制审批（变更窗口） | 重要：State 远端、配置快照 |
| Platform（平台） | RDS/CloudSQL、Kafka/Redis、对象存储桶、队列、Ingress/LB | 中 | Crossplane 或 控制器 | 周期/事件调谐（保守） | Correct（可纠偏） | 按环境（prod 需审批） | 变更前自动备份，RPO 15–60m |
| Application（应用） | 部署/服务、HPA、Config、短期 DNS 记录 | 高 | GitOps（K8s）/ 控制器 + Ansible | 持续调谐 | Correct | 自动 | 无需（或镜像可重建） |
| Ephemeral（短命） | 预发资源、沙箱账户、临时 Topic/队列、实验性存储 | 高 | 控制器（TTL/Owner） | 持续 + TTL | GC | 无 | 无（或最低） |

**原则：** 越“地基”，越保守（检测 > 纠偏 > 审批）；越贴近应用，越自动化（持续纠偏、快速回滚）。

以上矩阵被固化为平台默认策略，驱动策略路由、审批、备份、调谐频率等关键行为。

## 1. 总体架构

```mermaid
flowchart LR
  subgraph Git["Git Monorepo（唯一真相源）"]
    dirs[envs/\nmodules/\ncompositions/\nplaybooks/\npolicies/]
    pr[PR/Review/ChangeSet]
  end

  subgraph XCF["XcloudFlow 控制面（Go）"]
    API[API / CLI]
    Planner[Plan / Diff 生成器]
    Policy[Policy 引擎 (OPA / Conftest)]
    Router[策略路由 (Lifecycle×Frequency)]
    Orchestrator[编排器 (DAG / 依赖 / 并发 / 预算)]
    State[State 服务 (Postgres / SQLite / S3)]
    Secrets[秘钥管理 (Vault / KMS / SOPS)]
    Audit[审计 / 事件 / PR 注释]
    Obs[可观测性 (OTel 指标 / 追踪 / 日志)]
  end

  subgraph Engines["执行引擎层（可插拔）"]
    TF[Terraform Runner]
    PL[Pulumi Runner]
    PRV[原生 Providers (AWS/GCP/Azure/...)]
    PB[Playbook Runner (Ansible/SSH)]
    K8S[GitOps 引擎 (Argo/Flux) 或内置 Apply]
  end

  Git --> API --> Planner --> Policy --> Router --> Orchestrator
  Orchestrator --> TF
  Orchestrator --> PL
  Orchestrator --> PRV
  Orchestrator --> PB
  Orchestrator --> K8S
  Orchestrator --> State
  Secrets <---> Orchestrator
  Orchestrator --> Audit
  Orchestrator --> Obs
```

### 架构要点

- **策略路由**：根据 DSL 中的 `lifecycle`、`changeFrequency`、`reconcileStrategy`、`approvalPolicy`、`backupPolicy` 字段，将资源编排到 Terraform、Pulumi、原生 Provider、Playbook、K8s 引擎。
- **依赖图**：平台在 Planner 阶段构建 IaaS → PaaS → Apps → Config 的 DAG，保障拓扑顺序、并发限制及回滚流程。
- **多引擎一致体验**：统一的 Plan/Diff、策略校验、审批、执行回调、审计日志与 PR 回写体验，屏蔽底层引擎差异。

## 2. Git Monorepo 与 DSL

### 目录规范

```
repo-root/
  envs/           # 环境装配（prod/stage/dev 等），引用 compositions
  modules/        # Terraform/Pulumi 资源模块，与 playbook 角色解耦
  compositions/   # 组合模板，描述跨层级依赖与策略
  playbooks/      # Ansible/SSH 自研剧本，服务于 Config/Ops 层
  policies/       # OPA/Conftest 策略、审批规则、预算阈值
  .xcf.yaml       # 全局配置（状态后端、可观测性、凭证引用等）
```

### DSL 示例

```yaml
apiVersion: xcloudflow.io/v1alpha1
kind: Composition
metadata:
  name: payments-prod
spec:
  lifecycle: PaaS
  changeFrequency: medium
  modules:
    - ref: modules/aws/rds-cluster
      engine: terraform
      inputs:
        engineVersion: "15"
      policies:
        approval: environment
        backup: pre-change
  dependencies:
    - ref: compositions/foundation/network-prod
  reconcile:
    strategy: periodic
    interval: 6h
```

- `lifecycle` 与 `changeFrequency` 将触发策略路由，自动附带默认调谐、漂移、审批与备份策略。
- `engine` 可显式指定，也可使用 Router 依据矩阵自动决策。
- `policies` 支持覆盖默认策略，例如对生产环境 RDS 强制审批与备份。

## 3. 执行引擎层

| 引擎 | 主要职责 | 适配范围 | 集成特性 |
| --- | --- | --- | --- |
| Terraform Runner | 执行 IaaS/Foundation、K8s 基座的创建/变更 | AWS/GCP/Azure/阿里云/本地基础资源 | 计划缓存、状态后端、模块市场兼容 |
| Pulumi Runner | 支撑多语言编排、需要 SDK 逻辑的模块 | 中间件、跨云组合逻辑 | 共享状态与插件缓存、可嵌入 Policy as Code |
| Native Provider | 直接调用云厂商 API/SDK 或控制器 | 平台级服务（S3、RDS、Kafka） | 轻量、快速调谐、适合高频巡检 |
| Playbook Runner | 基于 Ansible/SSH 的配置与运维剧本 | OS/Agent/巡检/应急操作 | 幂等执行、日志追踪、批量并发 |
| K8s GitOps | Argo/Flux 或内置 Apply | 应用级资源、CRD、配置 | GitOps 对齐、实时漂移纠正 |

所有引擎通过统一的 Runner 接口接入控制面，支持：

- 标准化的 Plan/Diff 输出格式（JSON + Markdown 渲染），方便审批/审计。
- 统一的 Secrets 注入（Vault/KMS/SOPS），避免凭证散落。
- 事件回写到 Audit 与可观测性组件，实现跨引擎追踪。

### 3.1 Terraform / Pulumi Runner

- **输入**：接收 DSL/Composition 渲染后的模块路径与变量（`variables`）。
- **执行**：执行 `plan` 生成 diff（写回 PR 注释），审批通过后执行 `apply`。
- **State**：使用远端 S3/GCS 并通过 KMS 加密，或 Postgres Backend 管理锁与漂移记录。
- **漂移**：支持 `on_drift` 定期计划；Foundation 层默认仅检测不自动纠偏。

### 3.2 Native Providers（Go SDK）

- **适用资源**：RDS、S3、Redis、Kafka、Load Balancer、DNS 记录等 PaaS 服务。
- **调谐特性**：支持 `periodic`/`continuous` 调谐模式与自动纠偏，执行前触发快照。
- **幂等性**：使用 `ExternalID` 或资源 `Name` 作为幂等键，保障重试与蓝绿切换安全。
- **变更保护**：默认执行变更前快照或蓝绿切换策略，减少停机窗口。

### 3.3 Playbook Runner

- **输入**：引用 `playbooks/` 内剧本，结合 DSL 渲染出的主机清单（`hosts`/`labels`）与变量。
- **执行模式**：可调用 Ansible Runner，也支持原生 SSH 执行器，保证剧本幂等。
- **适用场景**：OS 配置、Agent 安装、探针/巡检、一次性修复任务。
- **安全保障**：支持 Dry-run、速率限流与失败回滚片段，避免大规模误操作。

### 3.4 K8s 引擎

- **集成模式**：
  - **GitOps 集成**：将 K8s 对象交给 Argo/Flux 等工具持续调谐。
  - **内置 Apply**：使用 `kubectl server-side apply --prune` 搭配 SOPS/Vault 管理敏感配置。
- **适用资源**：Deployment、Service、HPA、Job、Ingress、CRD 等应用层资源。
- **额外能力**：支持 Namespace/Cluster 级别隔离，结合策略执行快速回滚。

## 4. 编排与运行时策略

- **DAG 与锁**：Foundation 变更需获取独占锁，阻断下游自动化直到变更验证并解锁；PaaS 蓝绿切换与数据迁移按预算/批次执行（如每 30 分钟最多 2 个实例）。
- **频率驱动**：
  - Foundation：`on_drift` + 每日审计。
  - Platform：`periodic:5–30m`，失败自动 Backoff。
  - Applications：`continuous`，由 webhook 或 GitOps 触发。
  - Config：`on_change`，PR 合入即触发。
  - Ephemeral：`continuous`，附带 TTL GC。
- **审批与观察窗**：生产环境执行需审批，完成后进入观察窗，监测 Metrics/SLO 正常才关闭变更。
- **回写与审计**：计划、执行结果、快照 ID 及外部链接均写回 PR 评论；State 记录全量审计轨迹（who/when/what/result/diff/metrics）。

> StackFlow (Go Runner, MCP/skills/plugins, Agent mode, Cloud Run stateless) docs: `docs/stackflow/README.md`


## 5. 控制面协同

1. **API / CLI**：提供 PR 检查、手动调谐、回滚触发、预算查询等接口。
2. **Planner**：解析 DSL，生成资源依赖图与期望状态，调用相应 Runner 的 Plan 能力。
3. **Policy 引擎**：通过 OPA/Conftest 对 Plan 结果、输入参数、成本估算执行策略检查。
4. **策略路由（Router）**：参考决策矩阵与 DSL 自定义项，选择执行引擎、调度窗口、并发度及审批流程。
5. **编排器（Orchestrator）**：负责 DAG 调度、跨环境隔离、预算/速率限制、失败重试与回滚。
6. **State 服务**：集中管理状态、锁、事件，支持 PostgreSQL/S3/SQLite 后端。
7. **Secrets 管理**：统一加密解密流程，支持细粒度审计。
8. **审计与可观测性**：对每次 Plan/Apply 输出事件流，写回 PR 注释，并将指标、日志、追踪接入 OTel 生态。

## 6. 分层治理与策略化调谐

- **GitOps 即唯一真相源**：所有变更必须经过 PR -> Plan -> Policy -> 审批 -> Apply 流程；控制面在 Plan 阶段生成 ChangeSet 并留存。
- **调谐策略**：按矩阵默认值执行，可在 DSL 中覆盖。例：IaaS 使用 `on_drift` 检测 + 手动审批；Apps 使用 `continuous`，由 GitOps 控制器持续纠偏。
- **漂移处理**：IaaS/K8s 基座触发告警并生成待审批 ChangeSet；PaaS/Apps/Config 默认直接纠偏并记录。
- **门禁策略**：结合环境、资源类型与成本，定义审批人、自动化测试、预算阈值。Policy 引擎在 PR/Plan 期间强制执行。
- **备份策略**：在 Matrix 与 DSL 中声明，可调用快照 API、数据库备份、State Snapshot 或 Git Tag。

## 7. 运行策略

- **依赖拓扑**：按照 Foundation → Platform → Applications → Config 顺序执行，支持跨区域并发与故障域隔离。
- **窗口与预算**：Orchestrator 依据策略选择执行窗口（如工作日夜间），并对云预算/配额进行预检查。
- **回滚机制**：Plan 结果保留作为反向编排输入，控制面提供 `rollback` 指令自动执行逆向 ChangeSet。
- **审计闭环**：Plan、Approval、Apply、Drift、Rollback 事件均写入 Audit，并通过 Obs 输出指标与日志。

## 8. 落地路线图

1. **里程碑 1：多云基础能力**
   - 搭建 Git Monorepo、State 服务、Terraform/Pulumi Runner。
   - 实现 DSL 基础语法、决策矩阵映射、Plan/Apply 流程。
2. **里程碑 2：策略化治理**
   - 引入 OPA/Conftest、审批工作流、预算/配额校验。
   - 完成 Orchestrator 的 DAG 与并发调度，实现漂移检测与告警。
3. **里程碑 3：GitOps 与配置层自动化**
   - 集成 Argo/Flux 或内置 K8s Apply，接入 Playbook Runner。
   - 统一审计与可观测性输出，支持自动回滚。
4. **里程碑 4：企业级增强**
   - 打通 CMDB、工单系统、监控告警，沉淀标准报表。
   - 引入多租户、配额治理、成本追踪与更细粒度的策略模板。

通过以上步骤，XcloudFlow 能够在保持多云一致性的同时，实现分层治理、策略化调谐与快速变更的闭环体验。
