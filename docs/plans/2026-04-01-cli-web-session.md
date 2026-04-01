# CLI Web Session and UX Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为 LabKit 增加 `labkit web` 浏览器续登录能力，并用 `Lip Gloss` 重构 CLI 输出层的首批高价值命令。

**Architecture:** 服务端新增一次性网页登录 ticket 流程，CLI 用现有 Ed25519 签名身份申请 ticket，再打开浏览器消费 ticket 创建 session。CLI 输出层新增独立 presenter/UI 抽象，避免把样式逻辑塞进业务命令。

**Tech Stack:** Go, Cobra, existing Ed25519 signing/auth stack, PostgreSQL, Lip Gloss, optional browser opener helper

---

### Task 1: 写网页登录 ticket 的后端设计骨架

**Files:**
- Modify: `docs/reference/local-auth.md`
- Modify: `apps/api/internal/http/router.go`
- Create: `apps/api/internal/http/web_session_handler.go`
- Create: `apps/api/internal/service/websession/service.go`
- Test: `apps/api/internal/http/*_test.go`

**Step 1: Write the failing test**

补 API 测试，约束：
- `POST /api/web/session-ticket` 需要签名鉴权
- `GET /auth/session?ticket=...` 成功后会建立浏览器 session 并跳转

**Step 2: Run test to verify it fails**

Run: `go test ./apps/api/internal/http/...`
Expected: FAIL with missing routes / handlers

**Step 3: Write minimal implementation**

新增 handler 和 service 骨架，先让 ticket 申请和消费流存在。

**Step 4: Run test to verify it passes**

Run: `go test ./apps/api/internal/http/...`
Expected: PASS

**Step 5: Commit**

```bash
git add docs/reference/local-auth.md apps/api/internal/http apps/api/internal/service/websession
git commit -m "feat: add web session ticket flow"
```

### Task 2: 落数据库持久化 ticket

**Files:**
- Create: `db/migrations/*_web_session_tickets.up.sql`
- Create: `db/migrations/*_web_session_tickets.down.sql`
- Modify: `db/queries/*.sql`
- Modify: `packages/go/db/sqlc/*`
- Test: `packages/go/db/*_test.go`

**Step 1: Write the failing test**

补数据库或 service 测试，约束：
- ticket 可创建
- ticket 有 TTL
- ticket 只能消费一次

**Step 2: Run test to verify it fails**

Run: `go test ./packages/go/db/...`
Expected: FAIL with missing table/query

**Step 3: Write minimal implementation**

增加 migration、sqlc query、repo 调用。

**Step 4: Run test to verify it passes**

Run: `go test ./packages/go/db/...`
Expected: PASS

**Step 5: Commit**

```bash
git add db packages/go/db
git commit -m "feat: persist one-time web session tickets"
```

### Task 3: CLI 新增 `web` 命令

**Files:**
- Modify: `apps/cli/internal/commands/auth.go`
- Modify: `apps/cli/internal/commands/*.go`
- Create: `apps/cli/internal/commands/web.go`
- Test: `apps/cli/internal/commands/*_test.go`

**Step 1: Write the failing test**

补 CLI 测试，约束：
- `labkit web` 会申请 ticket
- 会生成目标 URL
- 会尝试打开浏览器
- 打不开时会输出 fallback URL

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/commands/...`
Expected: FAIL with missing command / behavior

**Step 3: Write minimal implementation**

新增 `web` 命令，复用现有签名请求能力。

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/commands/...`
Expected: PASS

**Step 5: Commit**

```bash
git add apps/cli/internal/commands
git commit -m "feat: add labkit web command"
```

### Task 4: CLI 输出层引入 Lip Gloss

**Files:**
- Modify: `apps/cli/go.mod`
- Create: `apps/cli/internal/ui/*.go`
- Modify: `apps/cli/internal/commands/auth.go`
- Modify: `apps/cli/internal/commands/keys.go`
- Modify: `apps/cli/internal/commands/lab_commands.go`
- Test: `apps/cli/internal/commands/*_test.go`

**Step 1: Write the failing test**

补命令输出测试，约束：
- `auth` 输出更结构化，不丢核心信息
- `submit`、`board`、`history`、`keys` 输出仍包含现有关键字段

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/commands/...`
Expected: FAIL because old plain output no longer matches new expectations

**Step 3: Write minimal implementation**

引入 `Lip Gloss` 和一个小型 `ui` 抽象，先重构高价值命令输出。

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/commands/...`
Expected: PASS

**Step 5: Commit**

```bash
git add apps/cli
git commit -m "feat: improve cli output with lip gloss"
```

### Task 5: 更新文档与本地使用路径

**Files:**
- Modify: `README.md`
- Modify: `docs/reference/local-auth.md`
- Modify: `docs/plans/2026-04-01-cli-web-session-design.md`

**Step 1: Write the failing test**

无代码测试；以文档核对为主。

**Step 2: Run test to verify it fails**

人工检查当前 README/local-auth 是否缺少 `labkit web`。

**Step 3: Write minimal implementation**

补使用说明、命令示例、续登录流程。

**Step 4: Run test to verify it passes**

Run: `rg -n "labkit web|session-ticket|/auth/session" README.md docs/reference`
Expected: matches found in the right files

**Step 5: Commit**

```bash
git add README.md docs/reference docs/plans/2026-04-01-cli-web-session-design.md
git commit -m "docs: document cli web session flow"
```

### Task 6: 全量验证

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

Expected: PASS

**Step 5: Commit**

```bash
git add .
git commit -m "chore: verify cli web session and ui upgrades"
```
