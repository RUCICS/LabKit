# Auth Provider Layer Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为 LabKit API 引入可替换的外部身份源 provider 层，让设备授权主流程不再直接依赖某一个学校 OAuth/CAS 的协议细节。

**Architecture:** 在 `apps/api/internal/service/auth` 中抽出统一 `Provider` 接口和标准身份模型，主流程只消费标准化的 `ExternalIdentity`。当前已工作的 OAuth 实现先迁成一个 provider，再让配置层通过 `LABKIT_AUTH_PROVIDER` 选择 provider，后续 provider 以并行实现方式接入。

**Tech Stack:** Go, existing auth service, env-based config, HTTP client, provider-specific unit tests, Go test

---

### Task 1: 定义 provider 抽象和标准身份模型

**Files:**
- Create: `apps/api/internal/service/auth/provider.go`
- Modify: `apps/api/internal/service/auth/device_flow.go`
- Test: `apps/api/internal/service/auth/device_flow_test.go`

**Step 1: Write the failing test**

在 `device_flow_test.go` 中增加一组测试，要求主流程不再依赖原始 OAuth profile 结构，而是依赖 fake provider 返回的标准身份对象：

- provider 返回 `StudentID` 时可以完成绑定
- provider 返回空 `StudentID` 时会失败

**Step 2: Run test to verify it fails**

Run: `go test ./apps/api/internal/service/auth -run 'TestHandleOAuthCallback.*Provider' -count=1`
Expected: FAIL because current `HandleOAuthCallback` still directly reads provider-specific fields

**Step 3: Write minimal implementation**

在 `provider.go` 中定义：

```go
type TokenSet struct {
    AccessToken string
    TokenType   string
    Scope       string
    Expiry      time.Time
}

type ExternalIdentity struct {
    Provider  string
    Subject   string
    StudentID string
    Name      string
    Email     string
}

type Provider interface {
    Name() string
    BuildAuthorizeURL(state string) (string, error)
    ExchangeCode(context.Context, string) (TokenSet, error)
    FetchIdentity(context.Context, TokenSet) (ExternalIdentity, error)
}
```

然后让 `device_flow.go` 改为依赖 `Provider`。

**Step 4: Run test to verify it passes**

Run: `go test ./apps/api/internal/service/auth -run 'TestHandleOAuthCallback.*Provider' -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add apps/api/internal/service/auth/provider.go apps/api/internal/service/auth/device_flow.go apps/api/internal/service/auth/device_flow_test.go
git commit -m "refactor(auth): introduce external identity provider interface"
```

### Task 2: 把当前已工作的 OAuth 实现迁成默认 provider

**Files:**
- Create: `apps/api/internal/service/auth/providers/cas_ruc.go`
- Modify: `apps/api/internal/service/auth/oauth_client.go`
- Modify: `apps/api/internal/service/auth/device_flow.go`
- Test: `apps/api/internal/service/auth/oauth_client_test.go`
- Test: `apps/api/internal/service/auth/device_flow_test.go`

**Step 1: Write the failing test**

补测试，覆盖：

- provider 能构造当前 CAS 风格 authorize URL
- provider 能换 token
- provider 能拉 profile 并映射出 `StudentID`

**Step 2: Run test to verify it fails**

Run: `go test ./apps/api/internal/service/auth -run 'TestCASRUCProvider|TestBeginDeviceVerification' -count=1`
Expected: FAIL because current OAuth client is not behind provider abstraction

**Step 3: Write minimal implementation**

把当前 `oauth_client.go` 的逻辑收进 `providers/cas_ruc.go`：

- `BuildAuthorizeURL`
- `ExchangeCode`
- `FetchIdentity`

并让旧行为保持兼容，包括现有 `scope=all`。

**Step 4: Run test to verify it passes**

Run: `go test ./apps/api/internal/service/auth -run 'TestCASRUCProvider|TestBeginDeviceVerification' -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add apps/api/internal/service/auth/providers/cas_ruc.go apps/api/internal/service/auth/oauth_client.go apps/api/internal/service/auth/device_flow.go apps/api/internal/service/auth/*test.go
git commit -m "refactor(auth): move current oauth flow behind cas provider"
```

### Task 3: 引入 provider factory 和配置选择

**Files:**
- Modify: `apps/api/internal/config/oauth.go`
- Create: `apps/api/internal/service/auth/providers/factory.go`
- Modify: `apps/api/cmd/labkit-api/main.go`
- Test: `apps/api/internal/config/oauth_test.go`

**Step 1: Write the failing test**

补配置测试，覆盖：

- `LABKIT_AUTH_PROVIDER=cas_ruc` 时能解析成功
- provider-required env 缺失时报清晰错误
- 未设置 provider 时有明确默认行为或报错

**Step 2: Run test to verify it fails**

Run: `go test ./apps/api/internal/config -count=1`
Expected: FAIL because current config still only returns one flat OAuth config struct

**Step 3: Write minimal implementation**

在配置层引入：

- `LABKIT_AUTH_PROVIDER`
- provider-specific config struct
- factory helper，根据 provider 名称构造 provider 实例

并在 `main.go` 里改成通过 factory 初始化 auth provider。

**Step 4: Run test to verify it passes**

Run:
- `go test ./apps/api/internal/config -count=1`
- `go test ./apps/api/cmd/labkit-api -count=1` if needed

Expected: PASS

**Step 5: Commit**

```bash
git add apps/api/internal/config apps/api/internal/service/auth/providers/factory.go apps/api/cmd/labkit-api/main.go
git commit -m "feat(auth): select auth provider from config"
```

### Task 4: 让 device_flow 彻底不再拼 provider-specific authorize URL

**Files:**
- Modify: `apps/api/internal/service/auth/device_flow.go`
- Test: `apps/api/internal/service/auth/device_flow_test.go`

**Step 1: Write the failing test**

补测试，明确 `BeginDeviceVerification` 只调用 provider，不再自行拼：

- state 从 repo 读取
- authorize URL 从 provider 返回
- `device_flow` 自己不再关心 `client_id`、`scope`、`redirect_uri`

**Step 2: Run test to verify it fails**

Run: `go test ./apps/api/internal/service/auth -run 'TestBeginDeviceVerification.*Provider' -count=1`
Expected: FAIL because current code still builds the query itself

**Step 3: Write minimal implementation**

把 `BeginDeviceVerification` 中的 URL 拼接逻辑移到 provider：

- 主流程只查 request
- 把 `state` 交给 provider
- provider 返回最终跳转地址

**Step 4: Run test to verify it passes**

Run: `go test ./apps/api/internal/service/auth -run 'TestBeginDeviceVerification.*Provider' -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add apps/api/internal/service/auth/device_flow.go apps/api/internal/service/auth/device_flow_test.go
git commit -m "refactor(auth): delegate authorize url generation to provider"
```

### Task 5: 新增第二个 provider 的最小骨架

**Files:**
- Create: `apps/api/internal/service/auth/providers/school_devcenter.go`
- Create: `apps/api/internal/service/auth/providers/school_devcenter_test.go`
- Modify: `apps/api/internal/config/oauth.go`

**Step 1: Write the failing test**

补测试，覆盖：

- `school_devcenter` provider 能构造 `oauth2/authorize`
- token 交换支持该 provider 所需的请求方式
- profile identity 能提取 `StudentID`

先只做最小骨架，不要求在生产环境马上启用。

**Step 2: Run test to verify it fails**

Run: `go test ./apps/api/internal/service/auth/providers -run 'TestSchoolDevcenter' -count=1`
Expected: FAIL because provider does not exist

**Step 3: Write minimal implementation**

实现最小可编译 provider：

- 支持 provider-specific env
- 映射成 `ExternalIdentity`
- 不改现有默认 provider 行为

**Step 4: Run test to verify it passes**

Run: `go test ./apps/api/internal/service/auth/providers -run 'TestSchoolDevcenter' -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add apps/api/internal/service/auth/providers/school_devcenter.go apps/api/internal/service/auth/providers/school_devcenter_test.go apps/api/internal/config/oauth.go
git commit -m "feat(auth): add school devcenter provider"
```

### Task 6: 更新部署模板和文档

**Files:**
- Modify: `deploy/.env.example`
- Modify: `deploy/.env.prod.example`
- Modify: `README.md`
- Modify: `docs/reference/local-auth.md`
- Test: `scripts/deploy_smoke_test.sh`

**Step 1: Write the failing test**

让部署 smoke test 先要求：

- `.env` 模板里出现 `LABKIT_AUTH_PROVIDER`
- provider-specific 必填项有检查

**Step 2: Run test to verify it fails**

Run: `bash scripts/deploy_smoke_test.sh`
Expected: FAIL because env example and docs still reflect old flat OAuth config model

**Step 3: Write minimal implementation**

更新：

- `.env.example`
- `.env.prod.example`
- README
- 本地认证文档

并在 smoke test 里验证新 env 形状。

**Step 4: Run test to verify it passes**

Run:
- `bash scripts/deploy_smoke_test.sh`
- `go test ./apps/api/... -count=1`

Expected: PASS

**Step 5: Commit**

```bash
git add deploy/.env.example deploy/.env.prod.example README.md docs/reference/local-auth.md scripts/deploy_smoke_test.sh
git commit -m "docs(auth): document provider-based auth configuration"
```

### Task 7: 全量回归

**Files:**
- No code changes required unless regressions are found

**Step 1: Run full targeted verification**

Run:

```bash
go test ./apps/api/... -count=1
bash scripts/deploy_smoke_test.sh
```

Expected: PASS

**Step 2: If regressions appear, fix one at a time**

只修回归，不顺手加新功能。

**Step 3: Commit final stabilization changes**

```bash
git add ...
git commit -m "test(auth): stabilize provider-layer rollout"
```
