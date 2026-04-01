# CLI UX Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 把 LabKit CLI 从“脚本式输出”重构成更接近 `uv` / `cargo` 的任务驱动型开发者工具，重点提升 `submit`、`auth`、`history`、`board` 的交互和终端体验。

**Architecture:** 先建立一个小型终端 UI 组件层，再把 `submit`、`auth`、`history`、`board` 接入。`submit` 默认等待到最终状态并输出结果卡；如果现有 API 不足以优雅轮询 submission 最终结果，先补服务端查询能力，再做 CLI 侧等待流。

**Tech Stack:** Go, Cobra, Lip Gloss, existing CLI signing stack, existing LabKit API

---

### Task 1: 盘点并补齐 `submit` 等待完成所需的状态查询能力

**Files:**
- Modify: `apps/api/internal/http/*`
- Modify: `apps/api/internal/service/personal/*` or `apps/api/internal/service/submissions/*`
- Test: `apps/api/internal/http/*_test.go`
- Test: `apps/api/internal/service/*_test.go`

**Step 1: Write the failing test**

先写服务端测试，明确 CLI 至少需要一种稳定方式按 `submission_id` 获取最终状态与结果：
- 如果已有 `GET /api/labs/{labID}/submissions/{submissionID}` 足够，就写测试锁定当前 contract
- 如果当前 detail API 字段不足，就先写缺失字段或缺失行为的失败测试

**Step 2: Run test to verify it fails**

Run: `go test ./apps/api/internal/http/... ./apps/api/internal/service/...`
Expected: FAIL at the missing status/detail behavior, or prove existing API is already sufficient

**Step 3: Write minimal implementation**

只做 `submit wait` 需要的最小服务端补足：
- 保证能稳定按 `submission_id` 查询
- 返回 CLI 需要的最终 `status/verdict/message/detail/scores`
- 不额外扩展无关 API

**Step 4: Run test to verify it passes**

Run: `go test ./apps/api/internal/http/... ./apps/api/internal/service/...`
Expected: PASS

**Step 5: Commit**

```bash
git add apps/api/internal/http apps/api/internal/service
git commit -m "feat: expose submission status for cli wait flow"
```

### Task 2: 建立新的 CLI UI 组件层

**Files:**
- Create: `apps/cli/internal/ui/theme.go`
- Create: `apps/cli/internal/ui/task.go`
- Create: `apps/cli/internal/ui/card.go`
- Create: `apps/cli/internal/ui/table.go`
- Create: `apps/cli/internal/ui/detail.go`
- Modify: `apps/cli/go.mod`
- Test: `apps/cli/internal/ui/*_test.go`

**Step 1: Write the failing test**

写 UI 层测试，锁定这些基础行为：
- task flow 能输出阶段、状态、完成态
- result card 能稳定输出键值信息
- compact table/list 不依赖大边框也能组织字段
- detail block 能承载多行失败信息

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/ui/...`
Expected: FAIL with missing package/files

**Step 3: Write minimal implementation**

用 `Lip Gloss` 搭一个小而稳定的组件层：
- `theme.go`：颜色、状态 badge、字重
- `task.go`：步骤/状态流
- `card.go`：结果卡
- `table.go`：紧凑表格/列表
- `detail.go`：多行 detail block

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/ui/...`
Expected: PASS

**Step 5: Commit**

```bash
git add apps/cli/go.mod apps/cli/go.sum apps/cli/internal/ui
git commit -m "feat(cli): add terminal ui primitives"
```

### Task 3: 重做 `submit` 为默认等待完成的任务流

**Files:**
- Modify: `apps/cli/internal/commands/lab_commands.go`
- Modify: `apps/cli/internal/commands/submit_test.go`
- Test: `apps/cli/internal/commands/*_test.go`

**Step 1: Write the failing test**

补 CLI 测试，锁定这些行为：
- `submit` 默认提交后轮询直到最终状态
- 轮询期间状态变化会反馈给用户
- 最终输出结果卡：`submission id / status / verdict / elapsed`
- `scored` 时显示所有 metric 的紧凑列表
- 失败时自动展开 `message/detail`
- `--detach` 或 `--no-wait` 时提交后立即退出
- `Ctrl+C` 中断等待时给出后续提示，不取消服务端任务

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/commands/...`
Expected: FAIL because current submit returns immediately and has no wait flow

**Step 3: Write minimal implementation**

在 `submit` 里接入新的 task flow：
- 本地步骤反馈：validate / fetch manifest / package / sign / upload
- 上传成功后用 `submission_id` 轮询最终状态
- 完成后渲染结果卡与 detail block
- 加 `--detach` / `--no-wait`
- 处理中断信号但不向服务端发送取消

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/commands/...`
Expected: PASS

**Step 5: Commit**

```bash
git add apps/cli/internal/commands/lab_commands.go apps/cli/internal/commands/submit_test.go
git commit -m "feat(cli): wait for final submission status by default"
```

### Task 4: 重做 `auth` 为分阶段授权流程

**Files:**
- Modify: `apps/cli/internal/commands/auth.go`
- Modify: `apps/cli/internal/commands/auth_test.go`

**Step 1: Write the failing test**

补 `auth` 输出测试，锁定这些行为：
- 显示阶段流：generate keypair / request authorization / waiting / authorized
- 显示 verification URL 与 user code
- 成功后显示 `student_id / key_id`
- opener/fallback 场景不丢失关键信息

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/commands/...`
Expected: FAIL because current auth still mostly是原始线性输出

**Step 3: Write minimal implementation**

把 `auth` 接入 task flow 和 result card：
- 生成 keypair
- 请求 device authorization
- 显示 verification URL / user code
- 等待 approval
- 输出绑定完成结果卡

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/commands/...`
Expected: PASS

**Step 5: Commit**

```bash
git add apps/cli/internal/commands/auth.go apps/cli/internal/commands/auth_test.go
git commit -m "feat(cli): redesign auth flow output"
```

### Task 5: 重做 `history` 为提交流视图

**Files:**
- Modify: `apps/cli/internal/commands/lab_commands.go`
- Modify: `apps/cli/internal/commands/submit_test.go`

**Step 1: Write the failing test**

补 `history` 输出测试，锁定这些行为：
- 默认输出是“最近提交流”，不是裸 tabwriter
- 每条记录突出 `submission id`、`status/verdict`、时间、核心分数或失败原因
- 最新一条更容易扫描
- 失败原因不会被完全埋掉

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/commands/...`
Expected: FAIL because current history still是原始表格

**Step 3: Write minimal implementation**

用 compact list / table 重构 `history`：
- submission short id
- status badge / verdict
- created/finished time
- scored 时的核心结果
- failure 时的 message 摘要

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/commands/...`
Expected: PASS

**Step 5: Commit**

```bash
git add apps/cli/internal/commands/lab_commands.go apps/cli/internal/commands/submit_test.go
git commit -m "feat(cli): redesign history output"
```

### Task 6: 重做 `board` 为排行榜快照

**Files:**
- Modify: `apps/cli/internal/commands/lab_commands.go`
- Modify: `apps/cli/internal/commands/submit_test.go`

**Step 1: Write the failing test**

补 `board` 输出测试，锁定这些行为：
- 顶部显示 lab / selected metric
- 排名和主 metric 更突出
- track 存在时清楚展示
- 输出仍保留可复制和可 grep 的字段

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/commands/...`
Expected: FAIL because current board still主要是 tabwriter 表格

**Step 3: Write minimal implementation**

用 compact table / result header 重构 `board`：
- lab name + metric summary
- top rows 排版优化
- 主 metric 强调
- 时间列层级收紧

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/commands/...`
Expected: PASS

**Step 5: Commit**

```bash
git add apps/cli/internal/commands/lab_commands.go apps/cli/internal/commands/submit_test.go
git commit -m "feat(cli): redesign board output"
```

### Task 7: 顺手统一 `keys / web / nick / track`

**Files:**
- Modify: `apps/cli/internal/commands/keys.go`
- Modify: `apps/cli/internal/commands/web.go`
- Modify: `apps/cli/internal/commands/lab_commands.go`
- Modify: `apps/cli/internal/commands/*_test.go`

**Step 1: Write the failing test**

补测试，锁定这些低成本一致化行为：
- `keys` 像设备清单，不再是长公钥挤坏表格
- `web` 输出和任务型命令风格统一
- `nick` / `track` 成功提示更干净

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/commands/...`
Expected: FAIL on updated output expectations

**Step 3: Write minimal implementation**

只做低成本一致化：
- `keys` 裁切长公钥，保留识别信息
- `web` 接入统一任务/结果样式
- `nick` / `track` 用统一成功提示

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/commands/...`
Expected: PASS

**Step 5: Commit**

```bash
git add apps/cli/internal/commands
git commit -m "feat(cli): unify secondary command output"
```

### Task 8: 更新文档

**Files:**
- Modify: `README.md`
- Modify: `docs/reference/local-auth.md`
- Modify: `docs/plans/2026-04-01-cli-ux-redesign-design.md`

**Step 1: Write the failing test**

无代码测试；以文档核对为主。

**Step 2: Run test to verify it fails**

Run: `rg -n "no-wait|detach|labkit web|history|board|submit" README.md docs/reference docs/plans/2026-04-01-cli-ux-redesign-design.md`
Expected: missing or outdated CLI UX descriptions

**Step 3: Write minimal implementation**

补 CLI 使用说明：
- `submit` 默认等待完成
- `--detach/--no-wait`
- `auth` / `web` / `history` / `board` 的新交互预期

**Step 4: Run test to verify it passes**

Run: `rg -n "no-wait|detach|labkit web|history|board|submit" README.md docs/reference docs/plans/2026-04-01-cli-ux-redesign-design.md`
Expected: matches found in the right files

**Step 5: Commit**

```bash
git add README.md docs/reference docs/plans/2026-04-01-cli-ux-redesign-design.md
git commit -m "docs: describe redesigned cli workflows"
```

### Task 9: 全量验证

**Files:**
- No code changes required unless verification fails

**Step 1: Write the failing test**

无新增。

**Step 2: Run test to verify it fails**

如有回归则在此发现。

**Step 3: Write minimal implementation**

仅在失败时补修。

**Step 4: Run test to verify it passes**

Run:
- `go test ./apps/api/...`
- `go test ./apps/cli/...`
- `go test ./packages/go/...`
- `cd apps/web && npm test`

Expected: PASS

**Step 5: Commit**

```bash
git add .
git commit -m "chore: verify redesigned cli experience"
```
