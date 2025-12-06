# SQL接口可用性检查：逻辑与设计概述

## 一、概述

MatrixOne Operator 中的 SQL 接口可用性检查是确保 CN（Compute Node）节点能够正常接受和处理客户端 SQL 请求的核心机制。该机制采用多层次、多维度的检查策略，结合 Kubernetes 原生能力与 MatrixOne 应用层状态，实现从底层 Pod 健康到上层 SQL 服务可用性的完整验证链路。

SQL 接口可用性检查贯穿于 CN 节点的整个生命周期：从 Pod 创建、启动、就绪判定，到运行时的状态同步、故障检测，再到升级、缩容等变更场景下的连接性保证。其设计目标是确保只有真正能够处理 SQL 请求的 CN 节点才会被标记为就绪状态，从而避免流量被路由到不可用的节点，保障集群的稳定性和数据的正确性。

## 二、架构设计

### 2.1 多层次检查架构

SQL 接口可用性检查采用分层架构，从下至上包括三个层次：

**第一层：Kubernetes Pod 级别检查**

在 Pod 级别，Operator 通过 Kubernetes 的 ReadinessGate 机制实现了自定义的就绪条件。每个 CN Pod 的 PodSpec 中都会添加一个名为 `matrixorigin.io/cn-store` 的 ReadinessGate。这意味着 Pod 只有在满足所有标准 Kubernetes 就绪条件（如容器就绪、Readiness Probe 通过）以及自定义的 CN Store 就绪条件后，才会被 Service 的 Endpoints 控制器纳入到服务的后端列表中。

ReadinessGate 的实现位于 `pkg/controllers/common/cnstore.go`，通过 `AddReadinessGate` 函数在构建 Pod 规范时动态添加。该机制与 Kubernetes 的 Readiness Probe 机制协同工作，但提供了应用层面的额外检查维度。

**第二层：HAKeeper 应用状态检查**

HAKeeper 是 MatrixOne 集群的协调中心，负责管理所有 Store（包括 CN Store）的注册、状态同步和元数据管理。Operator 通过 HAKeeper 的 RPC 接口（通过 `mocli` 客户端封装）来验证 CN Store 是否已成功注册到集群中，并更新其工作状态。

在 CN Store Controller（`pkg/controllers/cnstore/controller.go`）的协调循环中，Operator 会调用 `UpdateCNLabel` 或 `PatchCNStore` 方法向 HAKeeper 更新 CN Store 的状态为 `Working`。只有当这个操作成功时，才认为 CN Store 已经正确注册到集群中，能够参与查询处理。

**第三层：SQL 连接实际验证**

最上层的检查是通过实际的 SQL 连接来验证接口可用性。Operator 提供了一个专用的 SQL 客户端封装（`pkg/mosql/client.go`），使用 MySQL 协议连接到 CN 节点的 SQL 端口（默认 6001），执行 SQL 查询来验证连接性和基本功能。

该客户端采用单例模式的连接池设计，通过 `sync.Mutex` 保证线程安全，首次连接时会从 Kubernetes Secret 中获取凭证信息构建 DSN（Data Source Name）。每次查询都设置了 10 秒的超时时间，避免因网络问题导致的长时间阻塞。

### 2.2 核心组件

**mosql.Client 接口**

`mosql.Client` 是 SQL 接口检查的核心抽象，定义了两个主要方法：

- `Query(ctx context.Context, query string, args ...any)`: 执行任意 SQL 查询，返回结果集
- `GetServerConnection(ctx context.Context, uid string)`: 获取指定 CN 节点的当前连接数，通过查询 `system_metrics.server_connections` 系统表实现

`moClient` 是该接口的实现，内部维护一个 `sql.DB` 连接池，通过懒加载方式在首次使用时建立连接。连接信息（用户名、密码）从 Kubernetes Secret 中动态获取，支持通过 `matrixorigin.io/cluster-credential` 注解指定凭证 Secret。

**CNStoreReadiness 条件**

这是 Kubernetes Pod Condition 的一种扩展类型，专门用于表示 CN Store 的就绪状态。该条件的类型为 `matrixorigin.io/cn-store`，状态可以是 `True` 或 `False`，并包含描述性的消息说明当前状态。

Controller 通过 `patchCNReadiness` 方法更新该条件，只有在以下情况满足时才设置为 `True`：
1. CN Store 已成功注册到 HAKeeper
2. CN Store 的工作状态为 `Working`（而非 `Draining`）
3. 所有必要的元数据（如 Labels）已同步

**状态同步机制**

CN Store Controller 持续监控 CN Pod 的状态，通过 `syncStats` 方法定期同步 CN Store 的运行时指标，包括：
- Session Count：当前活跃的 SQL 会话数
- Pipeline Count：正在执行的查询管道数（需要 MOFeaturePipelineInfo 特性支持）
- Replica Count：该 CN 节点负责的数据副本数（需要 MOFeatureShardingMigration 特性支持）

这些指标通过查询 CN 节点的 `SHOW PROCESSLIST` 和系统表获得，并存储在 Pod 的注解中，用于后续的调度决策（如缩容时的 Pod 选择）。

## 三、检查流程与时机

### 3.1 Pod 创建与启动阶段

当一个新的 CN Pod 被创建时，检查流程按以下顺序进行：

1. **容器启动检查**：Kubernetes 首先验证容器是否成功启动，Readiness Probe 是否通过。CN Pod 的 Readiness Probe 通常配置为检查 SQL 端口（6001）的 TCP 连接。

2. **HAKeeper 注册检查**：CN Store Controller 检测到新的 Pod 后，会尝试通过 HAKeeper 客户端将 CN Store 注册到集群中。这个过程包括：
   - 获取 Pod 的 UUID（从 Pod 的注解或标签中读取）
   - 调用 `UpdateCNLabel` 更新 CN Store 的 Labels
   - 调用 `PatchCNStore` 设置 CN Store 的工作状态为 `Working`

3. **ReadinessGate 条件设置**：只有在 HAKeeper 注册成功后，Controller 才会将 `CNStoreReadiness` 条件设置为 `True`。此时，Pod 的所有 ReadinessGate 条件都已满足，Kubernetes 会将该 Pod 加入到 Service 的 Endpoints 中。

4. **SQL 连接验证（可选）**：在某些特定场景下（如初始化 Metric 用户时），Operator 会主动建立 SQL 连接来验证接口可用性。这个过程通常发生在 MatrixOneCluster Controller 的协调循环中。

### 3.2 运行时状态同步

CN Store 进入运行状态后，Controller 会定期执行状态同步操作：

**周期性统计同步**：每 `resyncInterval`（通常为 10 秒）执行一次 `syncStats`，从 CN 节点的系统表中获取当前的会话数、管道数等指标。这些指标用于：
- 评估节点的负载情况
- 在缩容操作中选择最合适的 Pod 进行删除（选择负载最低的）
- 判断节点是否可以安全回收（所有指标为 0）

**连接性诊断**：当 CN 节点处于 Draining 状态时，可以启用连接性诊断（通过 Pod 注解 `matrixorigin.io/diagnos-draining` 控制）。诊断功能会详细记录连接失败的原因，帮助排查问题。

**状态变更响应**：当检测到 CN Store 状态异常时（如 HAKeeper 中状态变为非 Working），Controller 会立即将 `CNStoreReadiness` 条件设置为 `False`，使 Pod 从 Service 的 Endpoints 中移除，避免流量继续路由到异常节点。

### 3.3 升级与变更场景

在 CN 节点升级或配置变更时，SQL 接口可用性检查确保变更过程的平滑进行：

**滚动更新期间的检查**：OpenKruise 的 CloneSet 在执行滚动更新时，会触发 PreDelete 和 InPlaceUpdate 的生命周期钩子。这些钩子中包含了 `CNDrainingFinalizer`，确保 CN Store 在被替换前先进入 Draining 状态，等待现有连接完成后再进行替换。

**In-Place 更新处理**：对于支持原地更新的配置变更（如 ConfigMap 更新），Kubernetes 不会重启 Pod，但 CN 服务可能需要重新加载配置。此时，ReadinessGate 机制确保只有配置加载成功、SQL 接口仍可正常响应后，Pod 才保持 Ready 状态。

**缩容前的连接检查**：在执行缩容操作时，Operator 会检查待删除 Pod 的连接数。只有当所有指标（Session Count、Pipeline Count、Replica Count）都为 0 时，才认为该节点可以安全移除。否则，会先触发 Draining 流程，等待连接迁移完成。

## 四、关键实现细节

### 4.1 凭证管理与安全性

SQL 连接需要认证信息，Operator 通过 Kubernetes Secret 管理这些敏感数据。默认情况下，使用 `MatrixOneCluster` 的 `status.credentialRef` 指定的 Secret，该 Secret 包含：
- `username`: 数据库用户名（默认为 "dump"）
- `password`: 数据库密码

SQL 客户端在首次连接时从 Secret 中读取凭证，并使用 DSN 格式构建连接字符串。所有凭证数据在内存中以明文形式短暂存在（仅用于构建 DSN），不进行持久化存储。

对于 Metric 查询场景，Operator 会创建专门的 Metric 用户（`mo_operator`）和角色（`metric_reader`），该角色仅授予 `system_metrics.*` 表的 SELECT 权限，遵循最小权限原则。

### 4.2 超时与重试策略

SQL 查询设置了明确的超时时间（`queryTimeout = 10s`），避免因网络问题或节点无响应导致的长时间阻塞。如果查询超时，会返回错误，Controller 会在下一次协调循环中重试。

对于 HAKeeper RPC 调用，也有相应的超时机制（通过 `mocli.ClientSet` 配置）。这些超时设置保证了即使部分组件异常，也不会阻塞整个协调流程。

### 4.3 错误处理与容错

检查过程中的错误处理遵循"宽松失败"原则：非关键检查的失败不会阻止 Pod 的正常运行。例如，`syncStats` 过程中的错误只会被记录到日志中，不会影响 Pod 的就绪状态。

但对于关键的检查（如 HAKeeper 注册），失败会导致 Pod 保持非就绪状态，直到问题解决。这种设计平衡了可用性和正确性。

### 4.4 版本兼容性

Operator 通过 `SemanticVersion` 管理 MatrixOne 的版本，某些检查功能需要特定版本支持：

- `MOFeaturePipelineInfo`: 支持查询 Pipeline Count
- `MOFeatureShardingMigration`: 支持查询 Replica Count
- `MOFeatureDiscoveryFixed`: 使用固定格式的 Discovery Address

对于不支持的版本，相关检查会被跳过或降级处理，保证向后兼容。

## 五、与其他机制的集成

### 5.1 与 Service 的集成

Kubernetes Service 通过 Endpoints 控制器自动管理后端 Pod 列表。只有当 Pod 满足所有 ReadinessGate 条件时，才会被加入到 Endpoints 中。这意味着 SQL 客户端通过 Service 访问时，只会连接到真正可用的 CN 节点。

CNSet Controller 会创建两种类型的 Service：
- Headless Service：用于 Pod 之间的直接通信和 DNS 解析
- ClusterIP Service：用于客户端访问，负载均衡到所有 Ready 的 Pod

### 5.2 与 CN Pool 的集成

在 CN Pool 模式下，CN 节点采用"池化"管理，节点可以被动态分配给不同的工作负载。SQL 接口可用性检查确保只有可用且空闲的 CN 节点才会被纳入池中，供 CNClaim 使用。

CNClaim Controller 在分配 CN 时，会检查 Pod 的 `CNPodPhaseLabel`，只选择处于 `Idle` 状态的 Pod。而 Pod 的状态转换（Idle → Bound → Idle）依赖于 SQL 接口可用性检查的结果。

### 5.3 与监控系统的集成

SQL 接口可用性检查产生的状态信息会通过多种方式暴露给监控系统：

- **Prometheus Metrics**：通过 `system_metrics` 系统表查询，可以获取 CN 节点的连接数、查询数等指标
- **Pod 注解**：运行时指标存储在 Pod 的注解中，外部系统可以通过 Kubernetes API 直接读取
- **Status 字段**：CNSet 的 Status 中包含 ReadyReplicas、Stores 等信息，反映整体健康状态

## 六、设计优势与局限性

### 6.1 设计优势

1. **多层验证保证可靠性**：从 Pod 级别到应用级别再到 SQL 级别的三层检查，确保只有真正可用的节点才会接收流量。

2. **自动化程度高**：整个过程完全自动化，无需人工干预，减少了运维负担。

3. **与 Kubernetes 原生机制深度集成**：充分利用 ReadinessGate、Condition 等 Kubernetes 特性，保证了与生态系统的良好兼容性。

4. **支持优雅变更**：在升级、缩容等场景下，通过 Draining 机制确保连接不中断，提升了用户体验。

### 6.2 局限性

1. **检查延迟**：多层检查带来了一定的延迟，从 Pod 启动到真正就绪可能需要数秒到数十秒。

2. **资源消耗**：定期状态同步会产生一定的网络和 CPU 开销，在高并发场景下可能成为瓶颈。

3. **依赖 HAKeeper**：HAKeeper 的可用性直接影响检查结果，如果 HAKeeper 异常，可能导致所有 CN 节点无法就绪。

4. **网络分区敏感**：在网络分区场景下，SQL 连接检查可能因网络不可达而失败，即使 CN 节点本身正常。

## 七、未来改进方向

1. **更细粒度的健康检查**：可以引入更细粒度的检查，如查询延迟、错误率等，实现更智能的流量路由。

2. **检查结果缓存**：对于频繁的检查请求，可以引入缓存机制，减少对 CN 节点的压力。

3. **自适应检查频率**：根据节点的稳定性和负载情况，动态调整检查频率。

4. **更丰富的诊断信息**：增强连接性诊断功能，提供更详细的错误信息和修复建议。

SQL 接口可用性检查作为 MatrixOne Operator 的核心机制之一，确保了集群的稳定性和可靠性。通过多层次的检查策略和与 Kubernetes 的深度集成，实现了从底层基础设施到上层应用服务的全方位保障。

