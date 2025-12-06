# MatrixOne Operator 生命周期管理文档大纲

## 1. 概述

### 1.1 文档目的
本文档详细描述 MatrixOne Operator 如何管理 MatrixOne 集群的完整生命周期，包括创建、健康检查、升级、故障发现与恢复、资源回收以及监控指标上报等核心功能。

### 1.2 架构概述
- Operator 架构简述
- Controller 架构（MatrixOneCluster、LogSet、DNSet、CNSet、ProxySet 等）
- 资源依赖关系

---

## 2. 集群创建 (Cluster Creation)

### 2.1 创建流程概览
- MatrixOneCluster CR 创建
- 协调流程（Reconciliation）

### 2.2 创建顺序与依赖
- **阶段 1：初始化凭证**
  - Root 凭证生成（InitRootCredential）
  - 凭证存储（Kubernetes Secret）
  
- **阶段 2：LogSet 创建**
  - HAKeeper Bootstrap
  - Discovery Service 创建
  - StatefulSet 创建
  - Gossip 配置初始化
  
- **阶段 3：DNSet 创建**
  - 依赖 LogSet Ready
  - DN StatefulSet 创建
  - 连接到 LogSet
  
- **阶段 4：CNSet 创建**
  - 支持多 CN 组（CNGroups）
  - CloneSet 创建
  - Headless Service 创建
  - 连接到 DNSet 和 LogSet
  
- **阶段 5：可选组件**
  - ProxySet（如配置）
  - WebUI（如配置）

### 2.3 从备份恢复
- RestoreJob 创建流程
- RestoreFrom 字段处理
- 数据恢复到共享存储

### 2.4 状态同步机制
- Status 字段更新
- Condition 条件管理
- Phase 状态转换

---

## 3. 健康检查与拨测 (Health Check & Monitoring)

### 3.1 Pod 级别健康检查
- **Kubernetes 原生探针**
  - Liveness Probe
  - Readiness Probe
  - Startup Probe

### 3.2 应用级别健康检查
- **LogSet 健康检查**
  - HAKeeper 状态检查
  - Store 可用性检查（通过 HAKeeper GetClusterInfo）
  - 故障超时检测（StoreFailureTimeout）
  
- **DNSet 健康检查**
  - Store 状态收集（CollectStoreStatus）
  - AvailableStores vs Desired Replicas
  
- **CNSet 健康检查**
  - CN Store 状态检查
  - CNState 注解检查
  - ReadyReplicas 检查

### 3.3 集群级别健康检查
- **MatrixOneCluster Ready 条件**
  - LogService Ready
  - DNSet Ready
  - CNGroups Ready
  - ProxySet Ready（如存在）
  
- **状态聚合**
  - Synced 条件（所有子资源已同步）
  - Ready 条件（所有子资源已就绪）
  - Phase 计算（Ready/NotReady）

### 3.4 定期重同步机制
- Resync Interval 配置
- ErrReSync 错误处理
- Watch 机制更新

### 3.5 连接性测试
- SQL 接口可用性检查（CN）
- HAKeeper 端点连接（LogSet）
- 服务发现（Discovery Service）

---

## 4. 集群升级 (Cluster Upgrade)

### 4.1 升级触发条件
- 镜像版本变更（Image Tag）
- 配置变更（ConfigMap）
- 资源规格变更（Resources）
- 环境变量变更（Env）
- 命令行参数变更（ServiceArgs）

### 4.2 Rolling Update 策略
- **LogSet 升级**
  - 基于 OpenKruise AdvancedStatefulSet
  - Leader 优先策略（按 Leader 数量排序）
  - 优雅 Leader Transfer
  - ReserveOrdinals 支持
  
- **DNSet 升级**
  - StatefulSet Rolling Update
  - 顺序更新（按 Pod 序号）
  
- **CNSet 升级**
  - CloneSet In-Place Update
  - MaxUnavailable / MaxSurge 配置
  - Pod Draining（PreDelete Hook）
  - 最小就绪时间（MinReadySeconds）

### 4.3 升级顺序
- 当前设计：各组件独立升级
- 未来扩展：支持有序升级策略（LogService → DN → CN）

### 4.4 版本管理
- SemanticVersion 管理
- OperatorVersion 标签
- 版本兼容性检查

### 4.5 升级过程中的故障处理
- 升级暂停机制（PauseUpdate）
- 升级失败回滚
- Pod 故障时的升级行为

### 4.6 In-Place 更新
- ConfigMap 热更新（GateInplaceConfigmapUpdate）
- 非破坏性配置变更
- ConfigSuffix 注解机制

---

## 5. 故障发现与恢复 (Failure Detection & Recovery)

### 5.1 故障发现机制
- **Kubernetes 层面**
  - Pod Phase 监控
  - Container 退出码
  - 资源耗尽（OOM）
  
- **应用层面**
  - HAKeeper 集群信息（LogSet）
  - Store 状态查询
  - 连接失败检测

### 5.2 故障分类
- **可 Pod 级别故障转移**
  - Node 故障 + 网络存储
  - Pod 重启失败但可恢复
  
- **不可 Pod 级别故障转移**
  - Node 故障 + 本地存储
  - 数据损坏
  - 持续重启失败

### 5.3 自动修复机制（Auto Healing）
- **容器级别**
  - 自动重启失败容器
  - Liveness Probe 触发重启
  
- **Pod 级别**
  - 无状态 Pod：创建新 Pod
  - 有状态 Pod：添加新 Pod + 保留原 Pod
  
- **Store 级别（LogSet/DNSet）**
  - 故障超时检测（StoreFailureTimeout）
  - 故障转移（Failover）
  - ReserveOrdinals 机制
  - 故障 Pod 清理（Orphan/Delete 策略）

### 5.4 LogSet 故障转移详细流程
- **Repair 操作**
  - 检查故障 Store 列表
  - 判断是否超过少数派限制
  - 添加 ReserveOrdinals
  - 更新 Gossip 配置
  
- **失败 Pod 策略**
  - FailedPodStrategyDelete：直接删除
  - FailedPodStrategyOrphan：标记为需要外部处理

### 5.5 故障限制与保护
- **少数派故障保护**
  - 超过半数故障时不自动修复
  - 等待人工介入
  
- **故障转移限制**
  - ReserveOrdinals 数量限制
  - 防止无限故障转移

### 5.6 故障恢复后的资源清理
- 过期 Pod 清理（GC）
- PVC 保留策略（PVCRetentionPolicy）
- S3 存储清理（S3Reclaim Feature）

---

## 6. 资源回收与删除 (Resource Cleanup & Deletion)

### 6.1 删除流程概览
- Finalizer 机制
- 优雅删除流程

### 6.2 删除顺序
- **CNSet 删除**
  - Draining 策略（TerminationPolicy）
  - 等待所有连接断开
  - Scale to Zero
  - CloneSet 和 Service 删除
  
- **DNSet 删除**
  - StatefulSet 删除
  - Headless Service 删除
  
- **LogSet 删除**
  - StatefulSet 删除
  - Discovery Service 删除
  
- **MatrixOneCluster 删除**
  - 删除所有子资源（LogSet、DNSet、CNSet、WebUI、ProxySet）
  - 等待子资源完全删除
  - 移除 Finalizer

### 6.3 数据保留策略
- **PVC 保留**
  - PVCRetentionPolicyDelete：删除 PVC
  - PVCRetentionPolicyRetain：保留 PVC
  
- **S3 存储保留**
  - S3RetentionPolicy 配置
  - Bucket Finalizer 机制

### 6.4 优雅终止
- **CNSet Draining**
  - PreDelete Lifecycle Hook
  - CNDrainingFinalizer
  - 连接迁移
  
- **终止宽限期**
  - TerminationGracePeriodSeconds
  - Pod 优雅关闭

### 6.5 清理验证
- 资源存在性检查
- Finalizer 移除条件
- 删除完成确认

---

## 7. 监控指标上报 (Metrics & Observability)

### 7.1 Operator 级别指标
- **Metrics Collector**
  - 注册资源类型（LogSet、DNSet、CNSet、MatrixOneCluster）
  - Prometheus 指标格式
  
- **Controller Metrics**
  - Reconciliation 次数
  - 错误计数
  - 延迟统计

### 7.2 集群级别指标
- **Metric Reader 初始化**
  - Metric 用户创建（mo_operator）
  - Metric 角色创建（metric_reader）
  - 权限授予（system_metrics.* SELECT）
  - Secret 管理（MetricCredential）

### 7.3 Prometheus 集成
- **服务发现**
  - PromDiscoverySchemePod：基于 Pod Labels
  - PromDiscoverySchemeService：基于 Service Annotations（默认）
  
- **Metric Service**
  - DNSet Metric Service 创建
  - Prometheus 注解配置
  - Metrics Port（默认端口）

### 7.4 应用指标采集
- **RPC 指标**
  - CN RPC Duration（CnRPCDuration）
  - HAKeeper RPC Duration（HAKeeperRPCDuration）
  
- **系统指标查询**
  - SQL 接口查询（system_metrics.*）
  - Server Connections 统计

### 7.5 状态指标
- **Resource Status**
  - Ready Replicas
  - Desired Replicas
  - Available Stores
  - Failed Stores
  
- **Condition 指标**
  - Ready Condition 状态
  - Synced Condition 状态
  - 条件持续时间

---

## 8. 扩展与高可用 (Scaling & High Availability)

### 8.1 水平扩展（Scale Out）
- **LogSet 扩展**
  - 增加 Replicas
  - 新 Pod 加入集群
  - Gossip 配置更新
  
- **DNSet 扩展**
  - 增加 Replicas
  - StatefulSet 扩缩容
  
- **CNSet 扩展**
  - 增加 Replicas
  - CloneSet 扩缩容
  - 无需 PVC 复用（DisablePVCReuse）

### 8.2 水平缩容（Scale In）
- **CNSet 缩容**
  - PodsToDelete 指定删除
  - Draining 处理
  - PVC 复用配置（ReusePVC）

### 8.3 垂直扩展（Scale Up）
- 资源规格变更
- 触发 Rolling Update

### 8.4 CN 池化（CN Pool）
- CNPool 资源
- CNClaim 机制
- 动态分配与回收

---

## 9. 配置管理 (Configuration Management)

### 9.1 配置来源
- ConfigMap 配置
- 环境变量配置
- 命令行参数（ServiceArgs）

### 9.2 配置更新
- ConfigMap 变更检测
- In-Place ConfigMap 更新
- 滚动更新触发

### 9.3 配置验证
- Webhook 验证
- Schema 验证
- 业务逻辑验证

---

## 10. 最佳实践与故障排查

### 10.1 创建最佳实践
- 资源规划
- 存储配置
- 网络配置

### 10.2 升级最佳实践
- 升级前备份
- 分阶段升级
- 监控升级过程

### 10.3 故障排查
- 常见故障场景
- 日志查看方法
- Status 字段解读
- 诊断工具使用

### 10.4 性能优化
- 资源限制配置
- 探针间隔调整
- 重同步频率优化

---

## 11. 附录

### 11.1 关键代码位置
- Controller 实现文件
- 工具函数位置
- 类型定义位置

### 11.2 相关 RFC 文档
- LogSet 设计文档
- Runtime 设计文档
- MatrixOneCluster 设计文档

### 11.3 术语表
- 关键术语定义
- 缩写说明

