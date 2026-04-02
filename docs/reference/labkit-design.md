# LabKit：ICS 课程打榜式 Lab 通用基础设施

## 1. 概述

### 1.1 定位与边界

LabKit 是一套面向 ICS 课程的通用打榜基础设施，服务于"有数值指标、有排名、有竞争"的 Lab。其核心价值是：课程组只需要编写一个 Docker 评测镜像和一份声明式配置文件，即可获得完整的认证、提交、评测、排行榜能力，无需重复建设。

LabKit **不处理**纯正确性评测（pass/fail 型 Lab）。这类 Lab 没有竞争维度，不需要排行榜，用传统的 autograder（如 Gradescope）即可。

### 1.2 架构分层

系统分为三层，每层之间通过明确的接口解耦：

```
┌──────────────────────────────────────────────────┐
│                  labkit CLI                       │
│  通用客户端：auth / submit / board / history      │
│  启动时从服务端拉取 Lab Manifest 驱动行为          │
├──────────────────────────────────────────────────┤
│                Platform API                       │
│  通用服务端：认证、提交收发、配额管理、排行榜      │
│  读取 Lab Manifest 驱动所有 lab-specific 行为      │
│  对 evaluator 的输出做校验、存储、展示             │
├──────────────────────────────────────────────────┤
│           Evaluator Protocol                      │
│  接口协议：                                       │
│    输入 = /submission/ 下的学生文件                │
│    输出 = stdout 最后一行 JSON                    │
│    退出 = 0 正常 / 非零 evaluator 故障            │
├──────────────────────────────────────────────────┤
│         Lab-Specific Evaluator Image              │
│  每个 Lab 独立的 Docker 镜像                      │
│  负责编译、正确性检查、跑分、聚合、输出最终分数     │
│                                                   │
│  例：schedlab-eval:2026sp                         │
│      cachesim-eval:2026sp                         │
│      malloc-eval:2026sp                           │
└──────────────────────────────────────────────────┘
```

**新开一个打榜 Lab 需要做的事：**

1. 编写一份 `lab.toml`（Lab Manifest）
2. 构建一个 evaluator Docker 镜像
3. 在 Platform 注册该 Lab
4. 给学生分发一个带 `--lab` 参数的 CLI wrapper

Platform 的代码完全不需要修改。

---

## 2. Lab Manifest

Lab Manifest 是 Lab 设计者与 Platform 之间的声明式契约。它描述 Platform 需要知道的所有信息，不描述 evaluator 的内部实现。

### 2.1 设计原则

- **Manifest 描述 contract，不描述 implementation。** 它告诉 Platform 期待什么输入/输出、怎么呈现结果，不管 evaluator 内部怎么编译、怎么跑分、怎么聚合。
- **常见情况短，特殊情况可表达。** 单指标 Lab 只需寥寥数行；多指标竞赛 Lab 可以写全。
- **字段的存在即有语义。** 不写的字段走默认值，不存在"写了但无意义"的字段。

### 2.2 完整 Schema

```toml
# ═══════════════════════════════════════════════════
#  Lab Manifest — 完整字段参考
# ═══════════════════════════════════════════════════

[lab]
id   = "colab-2026-p2"             # 唯一标识，URL-safe，不可变
name = "CoLab 调度器竞赛"           # 展示名
tags = {}                          # 可选，自由 KV 元数据
                                   # 例：{ course = "ics2", semester = "2026-spring" }

# ── 提交物 ─────────────────────────────────────────
# 声明学生需要提交的精确文件列表。
# CLI 端在上传前校验文件是否齐全，evaluator 可以假设
# /submission/ 下的文件与此列表严格一致。

[submit]
files    = ["scheduler.cpp"]       # 精确文件名列表，不支持 glob
max_size = "1MB"                   # 可选，per-file 大小上限，默认 "1MB"

# ── 评测容器 ───────────────────────────────────────
# Platform 对 evaluator 镜像的运行约束。
# 镜像内部的 entrypoint、编译流程等由镜像自身定义，
# 不出现在 manifest 中。

[eval]
image   = "registry.example.edu/colab-eval:2026sp"
timeout = 300                      # 可选，秒，默认 300
# 容器始终以 --network=none 运行，不可配置。

# ── 提交配额 ───────────────────────────────────────
# daily：每自然日（服务器时区）的提交上限。
# free：evaluator 返回这些 verdict 时不消耗次数。
#
# 可选的 verdict 值见 §3.2 Evaluator 输出协议。
# "error"（evaluator 自身故障）始终不消耗次数，
# 无需在此声明。

[quota]
daily = 3
free  = ["build_failed"]          # 默认 []

# 运行时补充语义：
# - `quota.daily` 按服务端 `LABKIT_QUOTA_TIMEZONE` 的自然日计算，
#   默认 `Asia/Shanghai`，不是宿主机 OS 时区。
# - submission 创建时先预占为 `pending`，评测完成后再结算成
#   `charged` 或 `free`。
# - `error` 永远免费，不需要在 `quota.free` 中重复声明。
# - CLI 会在上传前检查“是否与最近一次 submission 内容相同”；
#   命中时只做软确认，不会被服务端硬拦截。

# ── 指标 ───────────────────────────────────────────
# 声明 evaluator 会输出哪些数值指标。
# Platform 用这些声明来：
#   - 校验 evaluator 输出的 scores 字段
#   - 生成排行榜的列
#   - 确定排序方向
#
# id 必须与 evaluator 输出 JSON 的 scores key 严格一致。

[[metric]]
id   = "throughput"                # 必须，与 evaluator 输出对应
name = "吞吐量"                    # 可选，展示名，默认 = id
sort = "desc"                      # 必须，"desc" = 越高越好，"asc" = 越低越好
unit = "x"                         # 可选，展示后缀

[[metric]]
id   = "latency"
name = "延迟"
sort = "desc"
unit = "x"

[[metric]]
id   = "fairness"
name = "公平性"
sort = "desc"
unit = "x"

# ── 排行榜 ─────────────────────────────────────────

[board]
rank_by = "throughput"             # 可选，默认按哪个 metric 排名
                                   # 默认 = 第一个 [[metric]] 的 id
pick    = false                    # 可选，是否允许学生自选排名 metric
                                   # 默认 false；设为 true 时学生可通过
                                   # `labkit track <metric_id>` 切换

# ── 时间线 ─────────────────────────────────────────

[schedule]
visible = 2026-04-07T00:00:00+08:00  # 可选，Board 开始可见（空榜）
                                      # 默认 = open
open    = 2026-04-14T00:00:00+08:00  # 必须，开始接受提交
close   = 2026-06-01T23:59:59+08:00  # 必须，停止接受提交 + Board 冻结
```

### 2.3 Schema 字段速查

| Section | 字段 | 必须 | 默认值 | 说明 |
|---------|------|:----:|--------|------|
| `[lab]` | `id` | ✓ | — | 唯一标识，URL-safe |
| | `name` | ✓ | — | 展示名 |
| | `tags` | | `{}` | 自由 KV 元数据 |
| `[submit]` | `files` | ✓ | — | 精确文件名列表 |
| | `max_size` | | `"1MB"` | per-file 大小上限 |
| `[eval]` | `image` | ✓ | — | Docker 镜像地址 |
| | `timeout` | | `300` | 评测超时（秒） |
| `[quota]` | `daily` | ✓ | — | 每日提交上限 |
| | `free` | | `[]` | 不消耗次数的 verdict 列表 |
| `[[metric]]` | `id` | ✓ | — | 指标标识，对应 evaluator 输出 |
| | `name` | | `= id` | 展示名 |
| | `sort` | ✓ | — | `"desc"` 或 `"asc"` |
| | `unit` | | `""` | 展示后缀 |
| `[board]` | `rank_by` | | 第一个 metric | 默认排名指标 |
| | `pick` | | `false` | 学生是否可自选排名指标 |
| `[schedule]` | `visible` | | `= open` | Board 可见时间 |
| | `open` | ✓ | — | 开始接受提交 |
| | `close` | ✓ | — | 停止提交 + Board 冻结 |

### 2.4 时间线语义

```
visible            open                          close
  │                  │                              │
  ▼                  ▼                              ▼
  Board 可见         开始接受提交                    提交关闭
  (空榜)            Board 开始有数据                 Board 冻结
```

Platform 在各阶段的行为：

| 阶段 | Board | 提交 | 选赛道 |
|------|-------|------|--------|
| `< visible` | 404 | 拒绝 | — |
| `visible ~ open` | 可见（空） | 拒绝，提示"尚未开放" | — |
| `open ~ close` | 正常更新 | 正常接受 | 允许（若 `pick = true`） |
| `> close` | 只读，冻结 | 拒绝，提示"已截止" | 锁定 |

### 2.5 示例：不同类型的 Lab

**CoLab 调度器竞赛（三指标，学生自选赛道）：**

```toml
[lab]
id   = "colab-2026-p2"
name = "CoLab 调度器竞赛"
tags = { course = "ics2", semester = "2026-spring" }

[submit]
files = ["scheduler.cpp"]

[eval]
image   = "registry.example.edu/colab-eval:2026sp"
timeout = 300

[quota]
daily = 3
free  = ["build_failed"]

[[metric]]
id = "throughput"
name = "吞吐量"
sort = "desc"
unit = "x"

[[metric]]
id = "latency"
name = "延迟"
sort = "desc"
unit = "x"

[[metric]]
id = "fairness"
name = "公平性"
sort = "desc"
unit = "x"

[board]
rank_by = "throughput"
pick    = true

[schedule]
visible = 2026-04-07T00:00:00+08:00
open    = 2026-04-14T00:00:00+08:00
close   = 2026-06-01T23:59:59+08:00
```

**Malloc Lab（双指标 + 综合得分，固定排名）：**

```toml
[lab]
id   = "malloc-2026"
name = "Malloc 性能挑战"
tags = { course = "ics2", semester = "2026-spring" }

[submit]
files = ["mm.c"]

[eval]
image   = "registry.example.edu/malloc-eval:2026sp"
timeout = 180

[quota]
daily = 5
free  = ["build_failed"]

[[metric]]
id   = "throughput"
name = "吞吐量"
sort = "desc"
unit = "Kops/s"

[[metric]]
id   = "utilization"
name = "空间利用率"
sort = "desc"
unit = "%"

[[metric]]
id   = "perf_index"
name = "综合得分"
sort = "desc"

[board]
rank_by = "perf_index"
# pick 默认 false — 所有人统一按 perf_index 排名
# Board 上同时展示三列，学生可以看到各维度的表现

[schedule]
open  = 2026-04-01T00:00:00+08:00
close = 2026-05-15T23:59:59+08:00
```

注：`perf_index = 0.6 × throughput_norm + 0.4 × utilization` 这个公式由 evaluator 内部计算。Platform 不知道也不需要知道这个公式，它只收到三个 final 数字。

**Data Lab（单指标，越少越好）：**

```toml
[lab]
id   = "datalab-2026"
name = "Data Lab 代码高尔夫"

[submit]
files = ["bits.c"]

[eval]
image   = "registry.example.edu/datalab-eval:2026sp"
timeout = 60

[quota]
daily = 10
free  = ["build_failed", "rejected"]

[[metric]]
id   = "ops"
name = "总运算符数"
sort = "asc"

[board]
rank_by = "ops"

[schedule]
open  = 2026-03-01T00:00:00+08:00
close = 2026-04-15T23:59:59+08:00
```

最简形态：单文件、单指标、`sort = "asc"`。

---

## 3. Evaluator 协议

Evaluator 是 Platform 与 lab-specific 逻辑之间的唯一接口。它以 Docker 容器为边界，通过文件系统输入和 stdout JSON 输出与 Platform 通信。

### 3.1 输入

Platform 在启动容器前，将学生提交的文件复制到容器内的 `/submission/` 目录。文件名与 manifest 中 `[submit].files` 声明严格一致。

Evaluator 可以假设：

- `/submission/` 下的文件与 manifest 声明一致（Platform 已做校验）
- 容器无网络访问（`--network=none`）
- 容器有内存和 CPU 限制（由 Platform 配置）
- 每次运行独立，无状态

### 3.2 输出

Evaluator 的 **stdout 最后一行**必须是一个合法的 JSON 对象。最后一行之前的 stdout 内容（编译日志、测试进度等）由 Platform 忽略。

JSON 对象的结构：

```json
{
  "verdict":  "scored",
  "scores":   { "throughput": 1.82, "latency": 1.45, "fairness": 1.21 },
  "detail":   { "format": "markdown", "content": "..." },
  "message":  "编译警告: scheduler.cpp:42 unused variable"
}
```

**字段说明：**

| 字段 | 类型 | 何时必须 | 说明 |
|------|------|----------|------|
| `verdict` | string | 始终 | 终态，固定词表（见下） |
| `scores` | object | `verdict = "scored"` | key 对应 manifest 中 `[[metric]].id`，value 为浮点数 |
| `detail` | object | 可选 | 展示给提交者的结构化详情 |
| `message` | string | 可选 | 展示给提交者的纯文本诊断信息 |

**`verdict` 词表：**

| verdict | 语义 | 配额消耗 |
|---------|------|---------|
| `build_failed` | 编译/构建失败 | 查 `[quota].free` |
| `rejected` | 编译通过但未达到评分门槛（如正确性检查失败） | 查 `[quota].free` |
| `scored` | 正常出分 | 始终消耗 |
| `error` | evaluator 自身故障 | 始终不消耗 |

`build_failed` 与 `rejected` 的区别给 Lab 设计者灵活性：可以让编译失败免费但正确性失败收费（`free = ["build_failed"]`），也可以两者都免费（`free = ["build_failed", "rejected"]`）。

**`scores` 校验：** Platform 对照 manifest 中的 `[[metric]]` 声明严格校验 evaluator 输出的 `scores`。key 多了或少了均视为 `verdict: "error"`（evaluator bug，不计次数）。

**`detail` 结构：**

```json
{
  "format": "markdown",
  "content": "### Public Workloads\n\n| Scenario | Speedup | Weight |\n|---|---|---|\n| sustained_load | 1.92x | 1.0 |\n..."
}
```

`format` 仅允许 `"text"` 或 `"markdown"`。Platform 前端对 `"text"` 做等宽渲染（`<pre>`），对 `"markdown"` 做 Markdown 渲染。Platform 不解析 `content` 的结构，只负责渲染——给学生看多少信息完全由 evaluator 决定。

`message` 与 `detail` 的分工：`message` 是 verdict 不为 `scored` 时的诊断信息（编译错误、正确性失败原因），`detail` 是 `scored` 时的详细分析。两者可以共存（例如编译有 warning 但仍然通过），但典型情况下只出现一个。

### 3.3 Exit Code

| Exit Code | 含义 |
|-----------|------|
| `0` | JSON 有效。即使 verdict 是 `build_failed`，也是 evaluator 正常报告了失败。 |
| 非零 | Evaluator 崩溃或异常退出。Platform 等价处理为 `verdict: "error"`。 |
| `137` | 被 OOM killer 终止（Platform 标记为 `error`）。 |

超时由 Platform 外部处理：超过 `[eval].timeout` 秒后 Platform 直接 kill 容器，标记为 `error`。

### 3.4 Platform 端执行流程

```bash
# 1. 创建容器
docker create \
  --network=none \
  --memory=512m \
  --cpus=2 \
  $EVAL_IMAGE

# 2. 复制学生文件
for file in ${MANIFEST_FILES[@]}; do
  docker cp "$file" "$CONTAINER:/submission/$file"
done

# 3. 启动容器，捕获 stdout
timeout ${EVAL_TIMEOUT}s docker start -a "$CONTAINER" > output.txt 2>/dev/null

# 4. 取最后一行，解析 JSON
RESULT=$(tail -1 output.txt)

# 5. 销毁容器
docker rm -f "$CONTAINER"
```

"最后一行 stdout 是 JSON"的约定比"整个 stdout 是 JSON"更实用——evaluator 的编译和执行过程中难免有 stdout 输出（`make` 日志、测试进度等），要求整个 stdout 干净不现实。Platform 只取最后一行，解析失败则视为 `error`。

### 3.5 Evaluator 镜像的典型结构

```dockerfile
FROM ubuntu:24.04

# 工具链
RUN apt-get update && apt-get install -y build-essential ...

# 烘焙 Lab 框架、workload 定义、baseline 等
COPY engine/    /eval/engine/
COPY workloads/ /eval/workloads/
COPY scripts/   /eval/scripts/

ENTRYPOINT ["/eval/scripts/run.sh"]
```

`run.sh` 示例（以 CoLab 为例）：

```bash
#!/bin/bash
set -e

cd /eval

# ── Build ──────────────────────────────────────
if ! make student STUDENT_SRC=/submission/scheduler.cpp 2>/dev/null; then
    echo '{"verdict":"build_failed","message":"编译失败，请检查语法错误"}'
    exit 0
fi

# ── Gate ───────────────────────────────────────
GATE_RESULT=$(./run_gate 2>/dev/null)
if [ "$GATE_RESULT" != "pass" ]; then
    echo "{\"verdict\":\"rejected\",\"message\":\"正确性检查未通过: ${GATE_RESULT}\"}"
    exit 0
fi

# ── Score ──────────────────────────────────────
# run_benchmark 负责：
#   - 跑 public + hidden workloads
#   - 聚合分数（geomean、Jain's index 等）
#   - 生成 detail（Markdown 格式的 per-scenario breakdown）
#   - 输出最终 JSON 到 stdout 最后一行
./run_benchmark
```

注意：即使 build 或 gate 失败，evaluator 也 exit 0（正常报告失败）。只有 evaluator 自身出 bug 时才非零退出。

### 3.6 本地开发与调试

Lab 设计者可以在本地模拟 Platform 的行为来测试 evaluator：

```bash
#!/bin/bash
# local-eval.sh — Lab 设计者本地测试用
# 用法：./local-eval.sh scheduler.cpp

IMAGE="colab-eval:2026sp"
FILE="$1"

docker build -t "$IMAGE" .
docker run --rm --network=none \
  -v "$(realpath "$FILE")":/submission/"$(basename "$FILE")" \
  "$IMAGE"
```

这个脚本是 evaluator 仓库自带的开发工具，不属于 Platform。

---

## 4. 身份认证

认证机制分为两个阶段：一次性的设备授权绑定，以及日常提交时的签名验证。

### 4.1 Device Flow + Ed25519 签名

学生首次使用时，通过 Device Flow 完成学号与公钥的绑定：

```
CLI                              Server                          Browser
 │                                  │                                │
 │  1. 生成 Ed25519 密钥对           │                                │
 │     (~/.labkit/id_ed25519)       │                                │
 │                                  │                                │
 │  2. POST /api/device/authorize   │                                │
 │     {public_key}                 │                                │
 │ ──────────────────────────────► │                                │
 │                                  │  返回 {device_code,            │
 │  ◄────────────────────────────── │   user_code: "ABCD-1234",     │
 │                                  │   verification_url}            │
 │                                  │                                │
 │  3. 终端显示：                    │                                │
 │     请访问 https://...           │                                │
 │     输入验证码: ABCD-1234        │                                │
 │                                  │    4. 学生打开 /auth/device    │
 │                                  │  ◄────────────────────────────│
 │                                  │       页面把 user_code 交给 API │
 │                                  │       跳转学校 SSO 登录         │
 │                                  │  ────────────────────────────►│
 │                                  │       SSO 回调 + 输入 user_code│
 │                                  │  ◄────────────────────────────│
 │                                  │                                │
 │                                  │  5. 绑定 学号 ↔ public_key     │
 │                                  │                                │
 │  6. 轮询 POST /api/device/poll   │                                │
 │ ──────────────────────────────► │                                │
 │  ◄────────────────────────────── │  返回 {status: "ok"}           │
 │                                  │                                │
 │  ✓ 设备已授权                     │                                │
```

**多设备支持：** 一个学号可绑定多个公钥。学生在另一台机器上再跑一次 `labkit auth` 即可完成新设备授权。

**密钥丢失恢复：** 重新运行 `labkit auth` 绑定新公钥，旧公钥可通过 `labkit revoke` 手动管理。

**CSRF 防护：** 浏览器端重定向至学校 SSO 时，Platform 在授权 URL 中附带随机 `state` 参数（存入 `device_auth_requests.oauth_state`），SSO 回调时严格校验 `state` 一致性，防止跨站请求伪造和会话串线。

### 4.2 签名提交

设备授权完成后，SSO 彻底退场。日常提交时 CLI 用本地私钥签名：

```
CLI                                          Server
 │                                              │
 │  payload = {files, timestamp, nonce, lab_id} │
 │  signature = Ed25519_Sign(SHA256(payload))   │
 │                                              │
 │  POST /api/labs/{lab_id}/submit              │
 │  {payload, public_key, signature}            │
 │ ──────────────────────────────────────────► │
 │                                              │  查公钥 → 找到学号
 │                                              │  验签 + 检查 timestamp/nonce
 │                                              │  通过 → 入队评测
 │  ◄────────────────────────────────────────── │
 │  {submission_id, status: "queued"}           │
```

**防重放：** payload 中包含毫秒级 timestamp 和随机 nonce。服务端拒绝 timestamp 偏差超过 5 分钟的请求，并对 nonce 去重。

### 4.3 Fallback：预分发 Token

如果学校 SSO 对接不顺利，可退回 token 方案：

- 课前依据学号列表批量生成唯一 token（`stu_<16 bytes hex>`）
- 通过教务系统或邮件逐人分发
- CLI 使用 `Authorization: Bearer <token>` 认证
- 评测和排行榜逻辑不受影响

两种方案的切换仅涉及认证层。

---

## 5. 数据模型

Platform 同时支持多个 Lab，共享用户系统和认证体系。

### 5.1 Schema

```sql
-- ═══════════════════════════════════════════════
--  Lab 注册
-- ═══════════════════════════════════════════════

CREATE TABLE labs (
    id                  TEXT PRIMARY KEY,           -- lab.toml 中的 lab.id
    name                TEXT NOT NULL,
    manifest            JSONB NOT NULL,             -- 完整 lab.toml 内容（解析后）
    manifest_updated_at TIMESTAMP DEFAULT NOW(),    -- manifest 最后一次变更时间
    created_at          TIMESTAMP DEFAULT NOW()
);

-- ═══════════════════════════════════════════════
--  用户（跨 Lab 共享）
-- ═══════════════════════════════════════════════

CREATE TABLE users (
    id          SERIAL PRIMARY KEY,
    student_id  TEXT UNIQUE NOT NULL,       -- 学号，SSO 回调时写入
    created_at  TIMESTAMP DEFAULT NOW()
);

-- 一个学号可绑定多个公钥（多设备支持）
CREATE TABLE user_keys (
    id          SERIAL PRIMARY KEY,
    user_id     INTEGER REFERENCES users(id),
    public_key  TEXT UNIQUE NOT NULL,       -- Ed25519 公钥（base64）
    device_name TEXT DEFAULT 'unknown',
    created_at  TIMESTAMP DEFAULT NOW()
);

-- Device Flow 进行中的授权请求（短生命周期）
CREATE TABLE device_auth_requests (
    id              SERIAL PRIMARY KEY,
    device_code     TEXT UNIQUE NOT NULL,
    user_code       TEXT NOT NULL,          -- 短验证码，如 ABCD-1234
    public_key      TEXT NOT NULL,
    student_id      TEXT,                   -- SSO 回调后填入
    oauth_state     TEXT UNIQUE,            -- SSO 重定向时的 state 参数，防 CSRF
    status          TEXT DEFAULT 'pending'
                    CHECK(status IN ('pending', 'approved', 'expired')),
    expires_at      TIMESTAMP NOT NULL,
    created_at      TIMESTAMP DEFAULT NOW()
);

-- user_code 在 pending 请求中必须唯一，防止验证页歧义
CREATE UNIQUE INDEX idx_user_code_pending
    ON device_auth_requests (user_code)
    WHERE status = 'pending';

-- ═══════════════════════════════════════════════
--  每 Lab 的用户配置
-- ═══════════════════════════════════════════════

-- 用户在某个 Lab 中的展示设置
-- 只有参与过该 Lab（至少提交一次）的用户才会有记录
CREATE TABLE lab_profiles (
    user_id     INTEGER REFERENCES users(id),
    lab_id      TEXT REFERENCES labs(id),
    nickname    TEXT NOT NULL DEFAULT '匿名',
    track       TEXT,                       -- 声明的排名 metric
                                            -- NULL = 使用 manifest 默认值
    PRIMARY KEY (user_id, lab_id)
);

-- ═══════════════════════════════════════════════
--  提交与评分
-- ═══════════════════════════════════════════════

CREATE TABLE submissions (
    id              SERIAL PRIMARY KEY,
    user_id         INTEGER REFERENCES users(id),
    lab_id          TEXT REFERENCES labs(id),
    key_id          INTEGER REFERENCES user_keys(id),
    artifact_key    TEXT NOT NULL,               -- 提交物归档路径
                                                 -- 格式: {lab_id}/{user_id}/{id}.tar.gz
                                                 -- 存储在本地目录或对象存储
    content_hash    TEXT NOT NULL,               -- 提交物的 SHA-256（hex）
                                                 -- 用于去重校验和审计复现
    status          TEXT NOT NULL               -- 提交的生命周期状态：
                    CHECK(status IN (           --   queued     等待评测
                      'queued',                 --   running    评测中
                      'running',                --   done       evaluator 已返回结果
                      'done',                   --   timeout    超时
                      'timeout',                --   error      系统错误
                      'error'
                    )),
    verdict         TEXT,                       -- evaluator 返回的 verdict
                                                -- NULL 直到 status = done
    message         TEXT,                       -- evaluator 的诊断信息
    detail          JSONB,                      -- evaluator 的 detail 对象
    image_digest    TEXT,                       -- 评测时使用的镜像 digest (sha256:...)
    started_at      TIMESTAMP,                  -- 容器启动时间
    finished_at     TIMESTAMP,                  -- 容器退出时间
    created_at      TIMESTAMP DEFAULT NOW()     -- 提交入队时间
);

-- 分数：每个 submission × metric 一行
CREATE TABLE scores (
    submission_id  INTEGER REFERENCES submissions(id),
    metric_id      TEXT NOT NULL,           -- 对应 manifest [[metric]].id
    value          REAL NOT NULL,
    PRIMARY KEY (submission_id, metric_id)
);

-- ═══════════════════════════════════════════════
--  排行榜（物化视图，每次评测完成后更新）
-- ═══════════════════════════════════════════════

-- 记录每个用户在每个 Lab 上用于排名的 submission
-- 使用最新一次 verdict = "scored" 的提交（非历史最优）
CREATE TABLE leaderboard (
    user_id        INTEGER REFERENCES users(id),
    lab_id         TEXT REFERENCES labs(id),
    submission_id  INTEGER REFERENCES submissions(id),
    updated_at     TIMESTAMP,
    PRIMARY KEY (user_id, lab_id)
);

-- ═══════════════════════════════════════════════
--  Nonce 去重（短期，定期清理）
-- ═══════════════════════════════════════════════

CREATE TABLE used_nonces (
    nonce       TEXT PRIMARY KEY,
    created_at  TIMESTAMP DEFAULT NOW()
);
```

### 5.2 查询排行榜

```sql
-- 查询某个 Lab 的排行榜
-- 以 CoLab throughput 赛道为例
SELECT
    lp.nickname,
    lp.track AS declared_track,
    s_tput.value  AS throughput,
    s_lat.value   AS latency,
    s_fair.value  AS fairness,
    lb.updated_at
FROM leaderboard lb
JOIN lab_profiles lp  ON lp.user_id = lb.user_id AND lp.lab_id = lb.lab_id
LEFT JOIN scores s_tput ON s_tput.submission_id = lb.submission_id
                       AND s_tput.metric_id = 'throughput'
LEFT JOIN scores s_lat  ON s_lat.submission_id = lb.submission_id
                       AND s_lat.metric_id = 'latency'
LEFT JOIN scores s_fair ON s_fair.submission_id = lb.submission_id
                       AND s_fair.metric_id = 'fairness'
WHERE lb.lab_id = 'colab-2026-p2'
ORDER BY s_tput.value DESC;  -- 按 throughput 降序（sort = "desc"）
```

实际使用中，Platform 根据 manifest 的 `[[metric]]` 列表和 `[board].rank_by` 动态构建此查询，不需要硬编码列名。

### 5.3 关键设计决策

**为什么 `scores` 是独立表而不是 `submissions` 的 JSONB 列？**

独立表可以直接 JOIN 做排序和聚合查询，避免在 SQL 中解析 JSONB。排行榜查询是高频操作，结构化存储更高效。

**为什么 `leaderboard` 是独立表而不是视图？**

排行榜的更新策略是"最新一次 scored 提交"，这需要对 `submissions` 做窗口查询。物化为独立表后，排行榜查询是简单的 JOIN，不需要每次都扫描全部提交历史。更新时机明确：每次评测完成（`status = done` 且 `verdict = scored`）时刷新该用户在该 Lab 的 leaderboard 行。

**为什么 `lab_profiles.track` 允许 NULL？**

NULL 表示"使用 manifest 的默认值"。这样新用户参与 Lab 时不需要显式设置赛道。只有在 `[board].pick = true` 且学生主动切换时才写入具体值。

**为什么提交物要持久化归档？**

提交物归档（`artifact_key` + `content_hash`）服务于三个场景：reeval（§8.6 全量重跑需要恢复原始文件）、审计复现（申诉时验证评测结果可复现）、代码查重（MOSS 需要访问源文件）。不归档意味着容器销毁后提交物永久丢失，上述流程全部无法落地。

---

## 6. API

所有端点以 `/api` 为前缀。Lab-specific 端点以 `/api/labs/{lab_id}` 为前缀。

### 6.1 认证相关

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/device/authorize` | 无 | 发起设备授权 |
| POST | `/api/device/poll` | 无 | CLI 轮询授权状态 |
| GET | `/api/device/verify` | SSO | 设备授权 callback / OAuth redirect |
| GET | `/api/keys` | 签名 | 列出已绑定的公钥 |
| DELETE | `/api/keys/{id}` | 签名 | 撤销某个公钥 |

### 6.2 Lab 操作

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/labs` | 无 | 列出所有可见的 Lab |
| GET | `/api/labs/{lab_id}` | 无 | 获取 Lab 信息（manifest 的公开部分） |
| POST | `/api/labs/{lab_id}/submit` | 签名 | 提交代码 |
| GET | `/api/labs/{lab_id}/submissions/{id}` | 签名 | 查询某次提交结果 |
| GET | `/api/labs/{lab_id}/history` | 签名 | 查看自己的提交历史 |
| PUT | `/api/labs/{lab_id}/track` | 签名 | 切换声明赛道（需 `pick = true`） |
| PUT | `/api/labs/{lab_id}/nickname` | 签名 | 修改昵称 |
| GET | `/api/labs/{lab_id}/board` | 无 | 排行榜（公开），`?by=<metric_id>` 切换排序 |

### 6.3 管理端点

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/admin/labs` | Admin Token | 注册新 Lab（上传 lab.toml） |
| PUT | `/api/admin/labs/{lab_id}` | Admin Token | 更新 Lab 配置（受限，见下） |
| GET | `/api/admin/labs/{lab_id}/grades` | Admin Token | 导出成绩（学号 + 分数） |
| POST | `/api/admin/labs/{lab_id}/reeval` | Admin Token | 全量重跑（见 §8.6） |

**`PUT /api/admin/labs/{lab_id}` 的更新约束：** Platform 只允许原地更新不影响分数语义的字段。以下变更被视为结构性变更，API 应**拒绝**并要求新建 Lab 或走显式 migration：

- `[[metric]]` 的 id 集合发生变化（增删 metric）
- `[[metric]]` 的 `sort` 方向变化（desc ↔ asc）
- `[board].rank_by` 发生变化（无论新值是否已存在）

允许原地更新的字段包括：`lab.name`、`lab.tags`、`metric.name`、`metric.unit`、`quota.daily`、`quota.free`、`schedule.*`、`eval.image`、`eval.timeout`、`board.pick`。

---

## 7. CLI

### 7.1 命名与分发

CLI 工具统一命名为 `labkit`，通过 wrapper 脚本实现 Lab-specific branding：

```bash
# schedlab 只是 labkit 的 alias + 固定指向
alias schedlab='labkit --lab colab-2026-p2'
```

或者分发一个 wrapper 脚本：

```bash
#!/bin/bash
# schedlab — CoLab 2026 Phase 2 CLI
exec labkit --lab colab-2026-p2 "$@"
```

学生使用时感知不到 labkit 的存在，只看到 `schedlab`（或 `malloclab`、`datalab` 等）。

### 7.2 命令一览

```bash
# ── 设备授权 ───────────────────────────────────
labkit auth                         # 首次设备授权

# ── 密钥管理 ───────────────────────────────────
labkit keys                         # 列出已绑定设备
labkit revoke <n>                   # 撤销某个设备

# ── 提交 ───────────────────────────────────────
labkit submit <files...>            # 提交代码
                                    # CLI 对照 manifest 的 files 列表校验

# ── 排行榜 ─────────────────────────────────────
labkit board                        # 查看排行榜（默认排名 metric）
labkit board --by <metric_id>       # 按指定 metric 查看

# ── 赛道 / 昵称 ────────────────────────────────
labkit track <metric_id>            # 切换声明赛道（需 pick = true）
labkit nick <name>                  # 修改昵称

# ── 历史 ───────────────────────────────────────
labkit history                      # 查看提交历史
```

### 7.3 提交输出示例

```
$ schedlab submit scheduler.cpp
⣾ 编译中...
✓ 编译通过
⣾ 正确性检查...
✓ 通过
⣾ 评测中...

  Metric       Score
  throughput   1.82x
  latency      1.45x
  fairness     1.21x

  声明赛道: throughput
  今日剩余: 2/3
```

注：CLI 的输出格式（几列、列名、排序）由 manifest 的 `[[metric]]` 声明驱动。上面的输出是 CLI 读取 manifest 后动态生成的，不是硬编码的。

### 7.4 实现

CLI 是一个 Python 脚本，依赖仅 `requests` 和 `cryptography`（Ed25519），随 Lab 框架分发。

启动时 CLI 向 Platform 请求 `GET /api/labs/{lab_id}` 获取 manifest 信息（本地缓存，定期刷新），后续行为（文件校验、列展示等）由 manifest 驱动。

---

## 8. 评测流水线

### 8.1 整体流程

```
labkit submit
    → 签名验证
    → 配额检查
    → 归档提交物（计算 content_hash，存储至 artifact_key）
    → 入队
    → 启动 Docker 容器
    → 从归档恢复学生文件至 /submission/
    → 等待容器退出（或超时 kill）
    → 读取 stdout 最后一行 JSON
    → 校验 verdict 和 scores
    → 更新数据库
    → 刷新排行榜（若 verdict = scored）
```

### 8.2 提交物归档

Platform 在入队前将学生提交的文件打包归档，确保提交物持久化。这是 reeval（§8.6）和审计复现的基础。

归档路径格式：`{lab_id}/{user_id}/{submission_id}.tar.gz`

归档时同时计算整个包的 SHA-256 哈希，存入 `submissions.content_hash`。这个哈希用于：去重提示（检测连续提交内容未变化）、审计复现（验证归档文件未被篡改）。

存储后端可以是本地目录或对象存储（如 MinIO、S3），Platform 通过 `artifact_key` 统一寻址，不关心底层实现。

重复提交检测属于 Platform 的辅助提示能力，不引入新的 verdict。默认行为是：若检测到与最近一次提交 `content_hash` 相同，可向 CLI 提示"提交内容未变化"，但仍按正常流程入队评测，最终是否消耗配额仍由 evaluator 返回的 verdict 决定。

### 8.3 配额管理

- 每日配额按自然日（服务器时区）重置
- 消耗配额的判定时机：evaluator 返回后，根据 verdict 查 `[quota].free` 列表
- `verdict = "error"` 始终不消耗（不是学生的错）
- 编译失败或 rejected 是否消耗取决于 manifest 配置

### 8.4 Docker 评测容器

安全措施：

- `--network=none`：禁止网络访问
- `--memory=512m`：内存限制（可按需调整）
- 超时保护：Platform 在 `[eval].timeout` 秒后 kill 容器
- 容器跑完即销毁，不保留文件系统
- 仅读取 stdout 最后一行 JSON，其他输出丢弃

### 8.5 排行榜更新策略

- 使用**最新一次** `verdict = "scored"` 的提交成绩（非历史最优）
- 学生可随时切换声明赛道（若 `[board].pick = true`），`[schedule].close` 即锁定
- Board 同时展示该 Lab 所有 metric 的分数

### 8.6 学期中变更策略

Lab 进行中可能需要更新 manifest（调整配额、修改展示名等）或更新 evaluator 镜像（修复评分 bug）。

**Manifest 变更：始终按最新版本展示。** Manifest 的可变字段要么是展示性的（metric 的 `name`、`unit`），要么是策略性的（`quota.daily`、`schedule`），不影响已有分数的含义。所有历史提交和 Board 统一使用当前 manifest 渲染。结构性变更（增删 metric）本质上是一个新 Lab，应 bump `lab.id` 或做显式 migration。

**Evaluator 镜像变更：保留原始分数 + image digest 做审计。** 每条 submission 记录了评测时使用的 `image_digest`。两种处理策略：

- **小问题（不影响排名公平性）**——接受新旧 image 的分数共存于 Board，不做额外处理。
- **大问题（评分逻辑变更）**——课程组通过 `POST /api/admin/labs/{lab_id}/reeval` 触发全量重跑，用新 image 重新评测所有当前 leaderboard 上的提交。Platform 从每个用户的 leaderboard submission 的 `artifact_key` 恢复原始提交物，重新走评测流程。重跑时保留旧 submission 记录不动，为每个用户创建新的 submission 记录（verdict、scores、image_digest 均为新值），leaderboard 指向新 submission。

---

## 9. 排行榜展示

### 9.1 榜单结构

**每个 metric 一个视图。** Board 始终按单一 metric 排序，通过 `?by=<metric_id>` 参数切换视图。所有视图都展示该 Lab 的全部 metric 列，但排名顺序由当前选中的 metric 决定。

`pick = true` 时，学生声明的赛道只影响两件事：默认视图（CLI 的 `labkit board` 默认显示自己声明的赛道视图）和成绩归档（`/api/admin/labs/{lab_id}/grades` 按学生声明的赛道导出排名）。Board 本身不做"混排"——不存在一张表里不同行按不同 metric 排的情况。

`PUT /api/labs/{lab_id}/track` 只能写入当前 manifest 中存在的 `metric.id`，且仅当 `board.pick = true` 时允许；否则 Platform 返回 400。

以 CoLab 为例，查看 throughput 视图（`labkit board --by throughput`）：

| Rank | Nickname | Track | Throughput ▼ | Latency | Fairness | Updated |
|------|----------|-------|-------------|---------|----------|---------|
| 1 | 🐱 调度猫 | throughput | 2.31x | 1.12x | 0.98x | 04-28 14:22 |
| 2 | xyzzy | throughput | 2.15x | 1.45x | 1.21x | 04-27 09:41 |
| 3 | sched_master | throughput | 1.98x | 1.67x | 1.55x | 04-29 10:15 |
| 4 | 匿名 | latency | 1.45x | 1.89x | 1.32x | 04-30 08:00 |

- Track 列显示该学生声明的赛道（仅 `pick = true` 时展示此列）
- ▼ 标记当前排序 metric
- 所有学生都出现在所有视图中，但排名随视图变化

切换到 latency 视图（`labkit board --by latency`），同样的四个人，但排名重排：

| Rank | Nickname | Track | Throughput | Latency ▼ | Fairness | Updated |
|------|----------|-------|-----------|---------|----------|---------|
| 1 | 匿名 | latency | 1.45x | 1.89x | 1.32x | 04-30 08:00 |
| 2 | sched_master | throughput | 1.98x | 1.67x | 1.55x | 04-29 10:15 |
| 3 | xyzzy | throughput | 2.15x | 1.45x | 1.21x | 04-27 09:41 |
| 4 | 🐱 调度猫 | throughput | 2.31x | 1.12x | 0.98x | 04-28 14:22 |

以 Data Lab 为例，单 metric `pick = false`：

| Rank | Nickname | Ops ▲ | Updated |
|------|----------|-------|---------|
| 1 | bit_wizard | 42 | 03-15 10:00 |
| 2 | 匿名 | 58 | 03-14 16:30 |
| 3 | xor_fan | 63 | 03-16 09:22 |

单 metric Lab 没有视图切换，也没有 Track 列。▲ 表示 `sort = "asc"`（越小越好）。

### 9.2 访问方式

- CLI 查看：`labkit board`
- 静态网页：只读，无需登录即可围观
- Board API：`GET /api/labs/{lab_id}/board`，返回 JSON

### 9.3 显示名

- 学生可自定义昵称（`labkit nick <name>`）
- 默认昵称为"匿名"
- 后台可通过 `user_id` 对应到学号，便于成绩录入

---

## 10. 防作弊

### 10.1 防身份冒充

- Ed25519 签名验证确保每次提交均来自持有对应私钥的人
- 密钥绑定通过学校 SSO 完成，学号与公钥强关联
- 防重放：timestamp + nonce
- 主动私钥共享属于物理层面作弊，不在技术防范范围内

### 10.2 防代码抄袭

- 定期对提交代码跑 MOSS 或类似代码相似度检测
- 对分数异常接近的提交对做额外 diff 比对
- 提交历史可作为辅助证据——渐进式开发 vs. 一次性粘贴

### 10.3 防容器逃逸 / 信息泄露

- `--network=none` 禁止网络
- 超时保护
- 仅返回评测脚本产生的 JSON，学生代码的 stdout/stderr 不回传
- 容器跑完即销毁

---

## 11. 技术栈

| 组件 | 方案 | 说明 |
|------|------|------|
| 后端 API | FastAPI (Python) | 轻量 |
| 数据库 | PostgreSQL | 多 Lab 共享，JSONB 存 manifest |
| 评测隔离 | Docker | 每次提交一个容器 |
| 任务队列 | Redis + RQ（或进程队列） | 串行评测避免资源争抢 |
| CLI 工具 | Python（`cryptography` + `requests`） | 随 Lab 框架分发 |
| 榜单网页 | 静态 HTML + JSON API | 只读，无需登录 |
| SSO 对接 | 学校 CAS/OAuth | 仅 Device Flow 阶段使用 |

---

## 12. 开一个新 Lab 的 Checklist

1. **编写 `lab.toml`**——按 §2 的 schema 填写
2. **构建 evaluator 镜像**——实现 §3 的协议（输入 `/submission/`，输出 JSON）
3. **本地测试**——用 §3.6 的 `local-eval.sh` 验证 evaluator 行为
4. **注册 Lab**——`POST /api/admin/labs`，上传 `lab.toml`
5. **分发 CLI**——创建 wrapper 脚本（`schedlab`、`malloclab` 等）
6. **发布**——设置 `[schedule]` 时间线，学生即可开始使用
