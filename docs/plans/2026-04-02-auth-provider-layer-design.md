# Auth Provider Layer 设计

日期：2026-04-02

## 背景

当前 LabKit 的设备授权主流程已经能工作，但“外部身份源”的实现还直接耦合在 `apps/api/internal/service/auth` 里：

- `device_flow.go` 自己拼 OAuth authorize URL
- `oauth_client.go` 直接假设 token/profile 接口形状
- `HandleOAuthCallback` 直接假设 profile 里能拿到 `loginName`

这带来三个问题：

1. 身份源协议细节泄漏进主流程
   - 主流程不该理解 CAS、学校 OAuth、未来别的系统的字段差异

2. 新 provider 接入成本高
   - 任何差异都要修改现有设备授权编排逻辑

3. 配置模型不内聚
   - 当前一组 `LABKIT_OAUTH_*` 环境变量更像“一个固定 provider 的实现细节”，而不是“可切换的身份源”

## 目标

这一轮只拆“外部身份源 provider 层”，不重写设备授权编排本身。

目标是：

1. 把学校 OAuth / CAS / 未来别的登录源抽成独立 provider
2. 让 provider 自己负责把外部资料翻译成 LabKit 标准身份对象
3. 让 `device_flow` 只处理：
   - 设备请求状态
   - state 校验
   - code 回调编排
   - 本地用户/公钥绑定
4. 把 provider 选择和配置解析收敛成统一入口

## 非目标

这一轮不做：

- 重写 device auth 数据模型
- 重写浏览器 session 或 ticket 机制
- 引入完整 OIDC discovery
- 统一 admin 登录体系

## 方案选择

### A. 在现有 service 里继续加 `if provider == ...`

优点：

- 改动表面上最小

缺点：

- 主流程继续理解 provider-specific profile 结构
- provider 越多，条件分支越乱
- 配置解析和协议细节继续散落

不选。

### B. 抽独立 Provider 接口，由 provider 输出标准身份对象

优点：

- provider 负责“外部协议 -> 标准身份”
- 主流程不关心 `loginName`、`profiles[].stno`、`username` 之类差异
- 新 provider 只新增实现，不改主流程

缺点：

- 需要一次性收束配置和接口边界

结论：采用。

### C. 连设备授权编排一起彻底拆成更细 orchestration 层

优点：

- 长期最干净

缺点：

- 这一轮明显过重
- 风险是把现有已工作的绑定流程也一起打散

暂不采用。

## 架构边界

新的边界是：

- `device_flow`
  - 只负责 device auth 编排
  - 不理解 provider-specific profile shape

- `provider`
  - 负责构造授权地址
  - 负责 code -> token
  - 负责 token -> identity
  - 负责把外部用户资料转换成 LabKit 标准身份

- `config`
  - 负责解析“当前使用哪个 provider”以及 provider 对应配置

## 标准身份模型

建议定义统一结构：

```go
type ExternalIdentity struct {
    Provider  string
    Subject   string
    StudentID string
    Name      string
    Email     string
}
```

含义：

- `Provider`: provider 名称，例如 `cas_ruc`、`school_devcenter`
- `Subject`: provider 内部稳定标识，优先用外部系统最稳定的主键
- `StudentID`: LabKit 本地绑定使用的学号/工号
- `Name` / `Email`: 可选展示字段

关键约束：

- `StudentID` 的提取规则由 provider 自己负责
- 主流程只要求 `StudentID` 非空，不关心它从哪来

这是更内聚的边界。否则主流程会重新开始理解每个 provider 的原始字段。

## Provider 接口

建议 provider 接口形态如下：

```go
type Provider interface {
    Name() string
    BuildAuthorizeURL(state string) (string, error)
    ExchangeCode(ctx context.Context, code string) (TokenSet, error)
    FetchIdentity(ctx context.Context, token TokenSet) (ExternalIdentity, error)
}
```

配套 token 结构：

```go
type TokenSet struct {
    AccessToken string
    TokenType   string
    Scope       string
    Expiry      time.Time
}
```

说明：

- provider 可以只实际使用 `AccessToken`
- 其余字段保留给将来的 provider 差异

## Provider 目录布局

建议把实现落到：

- `apps/api/internal/service/auth/provider.go`
- `apps/api/internal/service/auth/providers/`

例如：

- `providers/cas_ruc.go`
- `providers/school_devcenter.go`
- `providers/factory.go`

主流程只依赖 `Provider` 接口，不依赖具体实现。

## 配置模型

当前散落的 `LABKIT_OAUTH_*` 更像“某一个 provider 的字段”，不适合继续直接暴露给主流程。

建议增加：

```text
LABKIT_AUTH_PROVIDER=cas_ruc
```

然后 provider-specific 配置由 factory 解析。

短期为了平滑迁移，可以保留现有 env 名称，但通过 provider-specific config struct 收口。例如：

### `cas_ruc`

- `LABKIT_OAUTH_CLIENT_ID`
- `LABKIT_OAUTH_CLIENT_SECRET`
- `LABKIT_OAUTH_REDIRECT_URL`
- `LABKIT_OAUTH_AUTHORIZE_URL`
- `LABKIT_OAUTH_TOKEN_URL`
- `LABKIT_OAUTH_PROFILE_URL`

### `school_devcenter`

可复用：

- `LABKIT_OAUTH_CLIENT_ID`
- `LABKIT_OAUTH_CLIENT_SECRET`
- `LABKIT_OAUTH_REDIRECT_URL`

另加：

- `LABKIT_OAUTH_USER_URL`
- `LABKIT_OAUTH_SCOPE`

如果 token 交换或 profile 拉取需要特殊行为，也由 provider 内部决定，不让主流程感知。

## 主流程改造方式

`device_flow.go` 应收敛成：

1. `BeginDeviceVerification`
   - 从 provider 拿 authorize URL
   - 不再自己拼 query 细节

2. `HandleOAuthCallback`
   - 调 provider `ExchangeCode`
   - 调 provider `FetchIdentity`
   - 校验 `identity.StudentID`
   - 用 `StudentID` 完成本地用户绑定

也就是说：

- `device_flow` 不再知道 `scope=all`
- 不再知道 token 是 form 换还是 Basic Auth
- 不再知道 profile 字段叫 `loginName` 还是 `profiles[].stno`

## 渐进迁移策略

为了降低风险，这轮建议分两步：

1. 先把当前已工作的 provider 抽出来，行为不变
2. 再新增第二个 provider 作为并行实现

这样可以确保：

- 现有生产/本地流程先不被打坏
- 新 provider 在测试和配置上独立落地

## 测试策略

测试要分层：

1. provider 单元测试
   - authorize URL 构造
   - token response 解析
   - identity 映射

2. `device_flow` 服务测试
   - 用 fake provider 验证主流程只依赖标准身份对象
   - 确认 `StudentID` 缺失时拒绝绑定

3. 配置测试
   - `LABKIT_AUTH_PROVIDER` 选择正确 provider
   - 缺少 provider-required env 时错误信息清晰

## 预期收益

完成后会得到：

- 更内聚的 provider 边界
- 更稳定的主流程
- 更可演化的学校登录源接入方式
- 更清晰的生产配置模型

最重要的是，以后切换学校 OAuth / CAS / 其他身份源时，不再需要反复触碰 device auth 主编排。
