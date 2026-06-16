# Upstream merge 验证报告：oasys-validator v1.9.0 → v1.9.1

日期：2026-06-16
分支：`merge-v1.9.1` → `debank`（PR #3）
镜像：`oasys-writer:amd64-354bedd9`（public ECR，PR build）
判定：**PASS — 零回归**

---

## 1. release 主要更新 + 为什么升 / 为什么可以不升

**结论先行：官方明确 "Updating to this version is not mandatory"（optional）；对 DeBank 的只读 RPC/indexer 副本部署形态，升级必要性 low。** 无强制网络硬分叉要求本版本（不升不会掉出共识），且本版核心改动多为 validator/miner-only 或默认关闭的 feature，与 DeBank 采集路径无关；官方修复的 glibc/基础镜像问题作用于上游 Dockerfile，DeBank 自有 `Dockerfile.debank` 不受影响（N/A）。

**但改动面不小**——v1.9.1 实质是一次 **BSC v1.6.7 完整上游 re-base（159 commits / 218 files）**，含：
- **BEP-590**：快速最终性投票聚合（`assembleVoteAttestation` 按 `SourceNumber` 过滤投票）——共识相邻
- **BEP-592 BAL（Block Access List）**：statedb/state_processor/blockchain 注入记录 hook，**默认关**（`EnableBAL` 默认 false，nil-guard），BAL 不进 block hash（共识中立）
- **EVM opcode-fusion**（super-instruction）：默认关
- **4 个安全修复**：secp256k1 坐标校验、ECIES RLPx invalid-curve、KZG proof 踢 peer、超长 storage key 拒绝（+ x/crypto 0.42→0.45）
- 官方 release notes 列的 3 项（token transfer plugin / BSC v1.6.7 sync / Amazon Linux 2 镜像）严重淡化了上述改动面——故走完整 merge + 实测流程确认无回归。

## 2. merge 冲突与影响面

**冲突**：仅 `go.mod` / `go.sum`（lockfile 类）。取上游侧 + 重新生成，**保留 fork 的 etcd `replace → v3.5.30` pin**（debank 既有，规避 etcd v3.6.0 versionpb protobuf-extension 50001 冲突）。其余 218 文件 git 自动合并，无业务代码冲突。

> 核心原则：对齐 upstream 的处理（含 fork 残留移除）零说明；本次没有需要保留并论证的新 fork patch——既有 DeBank patch（live tracer pipeline 挂接 + etcd pin）随合并自然保留，未受 v1.9.1 影响。

**全仓 build**：PASS。`go build ./...` 仅 2 个 `txfilter/plugindummy`·`plugintransfer` 包报 "function main is undeclared in the main package"——这是 `-buildmode=plugin` 包被当普通 exe 编的 **baseline 假阳性**（pre-merge debank 同样如此，Makefile 用 `-buildmode=plugin` 编它们），非 merge 问题。node 主体（cmd/geth + core + consensus + DeBank patch）全部编过。

**正向影响（upstream → 采集）：clean。** DeBank 采集是 **live tracer pipeline**（经 `tracing.Hooks` 挂 EVM），不订阅 `chainFeed`——v1.9.1 的 `ChainEvent` 结构扩展（新增 Receipts/Transactions）、incr-snapshot 等均与采集路径无交集。三个核心 hook（OnBlockStart/OnBlockDBStart/OnCommit）调用位置与时序未变；hook 实现文件（`core/tracing/hooks.go`、`core/state/statedb_hooked.go`、`core/types/receipt.go`）v1.9.1 未触碰。

**反向影响（DeBank patch 在新代码路径下）：无本次引入的风险。** 反向分析标出的 panic 风险点（`bc.logger.OnCommit` 无 nil-guard、`OnTxEnd` defer 在 txfilter 拦截下 receipt=nil）经**逐字节 baseline 对比确认全部是 debank pre-existing（pre-merge 既有，非本 merge 引入）**，且触发条件为 miner-only / 默认关，对只读副本 inert。记为 DeBank patch 独立 hardening TODO，不阻断本 merge。BEP-592 BAL 默认关时 statedb 注入点全 nil-guard、无 panic、无状态影响。

## 3. 部署情况

测试机部署 geth backup writer（从生产 writer 快照恢复，单 geth 进程 + live tracer pipeline）：
- 镜像 `oasys-writer:amd64-354bedd9`（PR build，public ECR）；entrypoint 直起 `/app/geth`（绕 runmode wrapper，避 argv doubling），command 照抄生产 sts。
- **is_backup（防生产污染，关键）**：`--vmtrace=pipeline --vmtrace.jsonconfig=...` 内 `"is_backup":true`（fixed-backup 模式，`etcd_endpoints` 为空 → 不连 etcd、不参与/不干扰生产选主）+ `"version"` 用 PR sha 隔离 topic/S3 前缀。代码级确认（`processor/push.go`）：backup 模式直接 `return` 跳过 kafka push；S3 上传由 `IsLeader()`(=false) gate；测试机无 AWS 凭证亦写不了。**对生产 etcd / kafka / S3 三通道全隔离，运行时日志实证（`Created in fixed mode, isBackup=true`）。**
- **p2p 身份清理**：首启前删 `<datadir>/geth/nodekey`，重生（新 enode，不与生产 writer 冲突）。
- compose 全文见附录 A。

## 4. 部署后测试情况

| 项 | 结果 |
|---|---|
| build | PASS（见 §2；plugin 包 baseline 假阳性除外） |
| 启动/同步 | 正常，0 restart，无 panic/FATAL；公网 bootnode 连上，**快照新鲜 → 近瞬间追平 lockstep（lag≈3）** |
| **hash 抽样** | **双段 40/40 MATCH**（追块段起始 20 块 + 近 head 20 块）vs 公网 RPC → block hash 含 stateRoot/receiptsRoot，逐块字节一致；节点稳定停在 canonical tip 亦证全段状态正确 |
| **debug_trace** | `debug_traceBlockByNumber`(callTracer) 与生产 leader writer **字节级 MATCH** |
| **eth_simulateV1 / eth_call** | parity MATCH（v1.9.1 的 simulate finalization 改动未破坏 parity） |
| opcode-optimize / BAL | 确认默认 **关**（compose 无对应 flag + 日志无启用迹象 + trace 与生产字节一致反证） |

**判定：PASS，v1.9.1 merge 对状态计算 / trace / simulate / call 输出零回归，与生产 v1.9.0 逐字节一致。**

> 验证范围说明：DeBank 集成是 live tracer pipeline（非自定义 RPC 网关——`eth_multiCall`/`pre_traceMany`/`trace_transaction` 是 leafage/jrpcx 特性，本链不实现）。backup 模式下 pipeline 真实运行（init 确认）但产物 transient（处理后删、backup 不上传），无持久产物逐字节比；输出正确性由 hash 40/40 + callTracer 字节一致 + 反向分析 hook 未动 间接确证。完整 S3 对象逐块比需 leader 模式，超出 backup/只读测试 scope。

## 5. 过程中暴露的其他问题（需后续跟进）

- **CI 镜像发布已切 public ECR**（`oasys-writer`，非旧私有 repo）——流水线的镜像坐标据此更新。
- 节点 prune（默认保留 ~128 块）→ `debug`/state 类 RPC 仅能查近 head 块（数百块前 "historical state not available"）；历史区间 trace 验证需 archive 节点。
- （DeBank patch 独立项，与本 release 无关）反向分析发现的 pre-existing nil-guard panic 风险（`bc.logger.OnCommit` / `OnTxEnd`）建议后续 hardening。

---

## 附录 A：测试 backup writer compose

```yaml
name: oas-writer-merge-v1-9-1
networks:
  oas-net: { driver: bridge, ipam: { config: [{ subnet: <10.x.y.0/24> }] } }
services:
  node:
    image: <public-ecr>/oasys-writer:amd64-354bedd9
    entrypoint: ["/app/geth"]
    command:
      - --config=/etc/config/config.toml
      - --datadir=/var/data
      - --syncmode=full
      - --cache=4096
      - --history.transactions=0
      - --rpc.allow-unprotected-txs
      - --metrics
      - --vmtrace=pipeline
      - --vmtrace.jsonconfig=${VMTRACE_JSONCONFIG}   # 内含 "is_backup":true + version=PR sha 隔离
    volumes:
      - <deploy>/data:/var/data                       # 快照恢复卷
      - <deploy>/config/config.toml:/etc/config/config.toml:ro
    ports: ["127.0.0.1:<rpc>:8545", "127.0.0.1:<pprof>:9260"]
    mem_limit: 12g
    logging: { driver: json-file, options: { max-size: "100m", max-file: "5" } }
# 预检：首启前删 data/geth/nodekey（p2p 重生）；端口/subnet lihe-dev 唯一；securityContext 照抄生产
```

> VMTRACE_JSONCONFIG 的 brokers/buckets/etcd 等 DeBank infra 端点已省略（is_backup=true 下不写，且本报告链匿名）。
