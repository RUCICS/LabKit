# CLI Local/Global Config Layering Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为 LabKit CLI 引入项目级 `.labkit/config.toml`、支持 `lab`/`server_url` 分层覆盖，并把全局配置从 JSON 全量切换到 TOML。

**Architecture:** `apps/cli/internal/config` 负责统一解析全局与本地 TOML 配置，并提供按优先级合并后的有效配置。命令层不再直接拼凑 `--lab`、环境变量和全局文件，而是通过集中入口读取最终配置。

**Tech Stack:** Go, Cobra, `github.com/BurntSushi/toml`, existing CLI config/auth code, Go tests

---

### Task 1: 用测试锁定 TOML 配置格式与本地配置发现

**Files:**
- Modify: `apps/cli/internal/config/config.go`
- Modify: `apps/cli/internal/config/config_test.go`

**Step 1: Write the failing test**

补 `apps/cli/internal/config/config_test.go`，覆盖：
- `Read()` 从 `config.toml` 读取 `server_url`、`lab`、`key_id`、`key_path`
- `Write()` 生成 TOML 而不是 JSON
- 从子目录向上查找最近的 `.labkit/config.toml`

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/config -v`
Expected: FAIL because current code only understands `config.json` and has no local config discovery

**Step 3: Write minimal implementation**

在 `config.go` 中：
- 新增 `Lab string`
- 把文件名改为 `config.toml`
- 用 `github.com/BurntSushi/toml` 解析和编码
- 新增项目配置查找函数，例如 `FindLocalConfig(startDir string)`

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/config -v`
Expected: PASS

**Step 5: Commit**

```bash
git add apps/cli/internal/config
git commit -m "feat: add toml config discovery"
```

### Task 2: 写失败测试，固定配置优先级合并规则

**Files:**
- Modify: `apps/cli/internal/commands/auth.go`
- Modify: `apps/cli/internal/commands/auth_test.go`
- Modify: `apps/cli/internal/commands/submit_test.go`

**Step 1: Write the failing test**

补命令层测试，覆盖：
- `--server-url` 覆盖 local/global config
- `LABKIT_SERVER_URL` 覆盖 local/global config
- `--lab` 覆盖 local/global config
- 没传 `--lab` 时，命令会从 `.labkit/config.toml` 取值

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/commands -run 'Test.*(Config|Lab|ServerURL)' -v`
Expected: FAIL because current commands only read global config plus flags/env, and `lab` is not in config

**Step 3: Write minimal implementation**

在 `apps/cli/internal/commands/auth.go` 和 `apps/cli/internal/commands/lab_commands.go` 中：
- 增加统一的“有效配置解析”入口
- 让命令层通过统一入口读取最终 `lab` / `server_url`
- 调整报错文案，明确可以通过 `.labkit/config.toml` 提供默认值

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/commands -run 'Test.*(Config|Lab|ServerURL)' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add apps/cli/internal/commands
git commit -m "feat: resolve layered cli config"
```

### Task 3: 把 auth 和写配置路径全部切到 TOML

**Files:**
- Modify: `apps/cli/internal/commands/auth.go`
- Modify: `apps/cli/internal/commands/auth_test.go`
- Modify: `apps/cli/cmd/labkit/main.go`

**Step 1: Write the failing test**

补 `auth` 相关测试，约束：
- `labkit auth` 写出的是 `config.toml`
- 写入结果包含 `server_url`、`key_id`、`key_path`
- 不再创建 `config.json`

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/commands -run 'TestAuth.*' -v`
Expected: FAIL because current implementation writes JSON config

**Step 3: Write minimal implementation**

更新 `config.Write()` 的调用预期和相关测试夹具，让认证链路只写 TOML。

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/commands -run 'TestAuth.*' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add apps/cli/internal/commands apps/cli/cmd/labkit
git commit -m "feat: write cli auth config as toml"
```

### Task 4: 给主要命令接上本地配置默认值

**Files:**
- Modify: `apps/cli/internal/commands/lab_commands.go`
- Modify: `apps/cli/internal/commands/keys.go`
- Modify: `apps/cli/internal/commands/web.go`
- Modify: `apps/cli/internal/commands/submit_test.go`
- Modify: `apps/cli/internal/commands/auth_test.go`

**Step 1: Write the failing test**

补命令集成测试，覆盖：
- 在仓库子目录执行 `submit` / `board` / `history` / `web` 时会向上找到 `.labkit/config.toml`
- `keys` / `revoke` 继续只依赖全局身份配置，不误读项目配置中的身份字段

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/commands -v`
Expected: FAIL because commands do not yet use local project config discovery consistently

**Step 3: Write minimal implementation**

统一各命令的配置入口，确保：
- `lab` 和 `server_url` 都走合并配置
- `key_id` / `key_path` 仍然来自全局配置

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/commands -v`
Expected: PASS

**Step 5: Commit**

```bash
git add apps/cli/internal/commands
git commit -m "feat: apply local project config to cli commands"
```

### Task 5: 更新文档与示例

**Files:**
- Modify: `README.md`
- Modify: `docs/reference/local-auth.md`
- Modify: `docs/plans/2026-04-01-cli-config-layering-design.md`

**Step 1: Write the failing test**

无代码测试；以文档核对为主。

**Step 2: Run test to verify it fails**

Run: `rg -n "config.json|config.toml|\\.labkit/config.toml|--lab|--server-url" README.md docs/reference`
Expected: current docs still mention old JSON-only model or omit project config

**Step 3: Write minimal implementation**

补文档，明确：
- 全局配置文件位置
- 本地配置文件位置
- 覆盖优先级
- `auth` 与项目配置的职责边界

**Step 4: Run test to verify it passes**

Run: `rg -n "config.toml|\\.labkit/config.toml" README.md docs/reference`
Expected: matches found in the right files

**Step 5: Commit**

```bash
git add README.md docs/reference docs/plans/2026-04-01-cli-config-layering-design.md
git commit -m "docs: document layered toml cli config"
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
- `go test ./apps/cli/internal/config -v`
- `go test ./apps/cli/internal/commands -v`
- `go test ./apps/cli/...`

Expected: PASS

**Step 5: Commit**

```bash
git add .
git commit -m "chore: verify layered toml cli config"
```
