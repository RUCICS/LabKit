# CLI Key Fingerprint and Optional Key Encryption Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 用 `key_fingerprint` 替换 CLI 协议里的 `key_id`，把全局配置升级成按服务器存储的 keyring，并在 `labkit auth` 中增加可选私钥加密。

**Architecture:** CLI 侧从本地私钥派生公钥和 fingerprint，不再依赖服务端返回的 `key_id` 作为请求标识；服务端按 fingerprint 查公钥再验签。全局配置改成 `servers.<origin>` 结构，每个服务器独立维护 `key_path`、fingerprint 和加密标记。认证命令在交互式终端中提供是否加密私钥的 prompt，并保留非交互 flag。

**Tech Stack:** Go, Cobra, Ed25519, SSH public key utilities, existing auth verifier, TOML config, PostgreSQL/sqlc, Go tests

---

### Task 1: 锁定 fingerprint 计算与加密私钥文件格式

**Files:**
- Modify: `apps/cli/internal/crypto/ed25519.go`
- Modify: `apps/cli/internal/crypto/*_test.go`

**Step 1: Write the failing test**

补测试，覆盖：
- 从 Ed25519 私钥推导公钥 fingerprint，输出 `SHA256:<base64>` 格式
- 明文私钥可读写
- 加密私钥可写入、读取，错误 passphrase 会失败

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/crypto -v`
Expected: FAIL because current crypto helper没有 fingerprint helper，也不支持加密私钥

**Step 3: Write minimal implementation**

在 `ed25519.go` 中：
- 增加 `PublicKeyFingerprint(...)`
- 增加支持加密/解密私钥的读写接口
- 保留当前明文读写路径，避免一次性破坏现有行为

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/crypto -v`
Expected: PASS

**Step 5: Commit**

```bash
git add apps/cli/internal/crypto
git commit -m "feat: add key fingerprint and optional key encryption"
```

### Task 2: 把全局配置改成多服务器 keyring

**Files:**
- Modify: `apps/cli/internal/config/config.go`
- Modify: `apps/cli/internal/config/config_test.go`

**Step 1: Write the failing test**

补配置测试，覆盖：
- `config.toml` 支持 `default_server_url`
- 支持 `servers.<origin>.key_path`
- 支持 `servers.<origin>.key_fingerprint`
- 支持 `servers.<origin>.encrypted`
- 项目配置 `.labkit/config.toml` 仍然只读 `server_url` / `lab`

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/config -v`
Expected: FAIL because current config model still是扁平结构

**Step 3: Write minimal implementation**

重构 `config.Config`：
- 区分全局配置和项目配置的字段职责
- 提供按 server origin 读取/更新单条 keyring entry 的 helper
- 保留本地配置向上查找逻辑

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/config -v`
Expected: PASS

**Step 5: Commit**

```bash
git add apps/cli/internal/config
git commit -m "feat: add multi-server keyring config"
```

### Task 3: 先写失败测试，把 CLI 协议从 key_id 切到 key_fingerprint

**Files:**
- Modify: `apps/cli/internal/commands/auth_test.go`
- Modify: `apps/cli/internal/commands/submit_test.go`
- Modify: `apps/cli/internal/commands/web_test.go`
- Modify: `apps/api/internal/http/*_test.go`
- Modify: `apps/api/internal/service/personal/*_test.go`

**Step 1: Write the failing test**

补测试，覆盖：
- CLI 请求发送 `X-LabKit-Key-Fingerprint`
- 不再发送 `X-LabKit-Key-ID`
- 服务端认证按 fingerprint 查 key 并验签
- 原有个人命令、提交、web session 都能通过新头部认证

**Step 2: Run test to verify it fails**

Run:
- `go test ./apps/cli/internal/commands -v`
- `go test ./apps/api/internal/http -v`
- `go test ./apps/api/internal/service/personal -v`

Expected: FAIL because current auth path still依赖 `X-LabKit-Key-ID`

**Step 3: Write minimal implementation**

更新：
- CLI 签名请求头
- HTTP 解析逻辑
- personal auth service 的 `AuthInput`
- repository lookup，从按 ID 查改为按 fingerprint / public key 查

**Step 4: Run test to verify it passes**

Run:
- `go test ./apps/cli/internal/commands -v`
- `go test ./apps/api/internal/http -v`
- `go test ./apps/api/internal/service/personal -v`

Expected: PASS

**Step 5: Commit**

```bash
git add apps/cli/internal/commands apps/api/internal/http apps/api/internal/service/personal
git commit -m "feat: authenticate signed requests by key fingerprint"
```

### Task 4: 调整 auth 流程和多服务器 key 复用/轮换行为

**Files:**
- Modify: `apps/cli/internal/commands/auth.go`
- Modify: `apps/cli/internal/commands/auth_test.go`
- Modify: `apps/cli/cmd/labkit/main.go`

**Step 1: Write the failing test**

补测试，覆盖：
- 同一 server 首次 `auth` 会生成新 key
- 同一 server 再次 `auth` 默认复用已有 key
- `--rotate-key` 会生成新 key
- 开头会提示当前是复用还是新建，并显示 fingerprint

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/commands -run 'TestAuth.*' -v`
Expected: FAIL because current `auth` 每次都按扁平配置写 key，没有 keyring 和复用/轮换逻辑

**Step 3: Write minimal implementation**

在 `auth.go` 中：
- 先解析目标 server
- 从 keyring 读取已有 entry
- 没有则新建；有则复用；指定 `--rotate-key` 则轮换
- 更新 keyring entry 的 `key_path` / fingerprint / encrypted

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/commands -run 'TestAuth.*' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add apps/cli/internal/commands apps/cli/cmd/labkit
git commit -m "feat: reuse or rotate per-server auth keys"
```

### Task 5: 给 auth 增加可选私钥加密 prompt 和非交互 flag

**Files:**
- Modify: `apps/cli/internal/commands/auth.go`
- Modify: `apps/cli/internal/commands/auth_test.go`
- Modify: `apps/cli/internal/output/*` if needed

**Step 1: Write the failing test**

补测试，覆盖：
- 交互 TTY 下会询问是否加密私钥
- `--encrypt` / `--no-encrypt` 会跳过 prompt
- 非交互环境缺少显式选择时，行为清晰且可测试
- 加密私钥后，后续命令读取时会提示解密

**Step 2: Run test to verify it fails**

Run: `go test ./apps/cli/internal/commands -run 'TestAuth.*Encrypt|Test.*EncryptedKey' -v`
Expected: FAIL because current auth 没有 prompt、flag 和加密读写流程

**Step 3: Write minimal implementation**

在 `auth.go` 中：
- 注入 prompt / passphrase reader 依赖，保持可测试
- TTY 下默认 prompt
- 支持 `--encrypt` / `--no-encrypt`
- 让后续读取私钥时能按需解密

**Step 4: Run test to verify it passes**

Run: `go test ./apps/cli/internal/commands -run 'TestAuth.*Encrypt|Test.*EncryptedKey' -v`
Expected: PASS

**Step 5: Commit**

```bash
git add apps/cli/internal/commands
git commit -m "feat: prompt for optional private key encryption"
```

### Task 6: 更新文档与迁移说明

**Files:**
- Modify: `README.md`
- Modify: `docs/reference/local-auth.md`
- Modify: `docs/plans/2026-04-01-cli-key-fingerprint-design.md`

**Step 1: Write the failing test**

无代码测试；以文档核对为主。

**Step 2: Run test to verify it fails**

Run: `rg -n "key_id|key_fingerprint|rotate-key|encrypt|\\.labkit/config.toml|default_server_url|servers\\." README.md docs/reference`
Expected: current docs missing new protocol/config/key encryption behavior

**Step 3: Write minimal implementation**

更新文档，明确：
- 请求头改成 fingerprint
- 全局 keyring 结构
- 每服务器独立 key
- `auth` 默认复用、`--rotate-key` 轮换
- 私钥加密 prompt 与非交互 flag

**Step 4: Run test to verify it passes**

Run: `rg -n "key_fingerprint|rotate-key|encrypt|default_server_url|servers\\." README.md docs/reference`
Expected: matches found in the right files

**Step 5: Commit**

```bash
git add README.md docs/reference docs/plans/2026-04-01-cli-key-fingerprint-design.md
git commit -m "docs: describe fingerprint auth and optional key encryption"
```

### Task 7: 全量验证

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
- `go test ./apps/cli/internal/crypto -v`
- `go test ./apps/cli/internal/config -v`
- `go test ./apps/cli/internal/commands -v`
- `go test ./apps/api/internal/http -v`
- `go test ./apps/api/internal/service/personal -v`
- `go test ./apps/cli/... ./apps/api/...`

Expected: PASS

**Step 5: Commit**

```bash
git add .
git commit -m "chore: verify fingerprint auth and key encryption flow"
```
