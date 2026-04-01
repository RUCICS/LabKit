# Device Auth Web Surface Design

## Goal

把设备授权的可见页面从 API callback endpoint 中拆出来，搬到正式的 web app 路由 `/auth/device`，同时把 `/api/device/verify` 收回成纯 redirect/callback handler。

## Problem

当前实现把三种职责混在 `GET /api/device/verify`：

- 渲染输入 `user_code` 的 HTML 页面
- 收到 `user_code` 后跳转到学校 OAuth
- 收到 `code/state` 后处理 OAuth callback

这在功能上可用，但产品边界和视觉质量都很差：

- 用户看到的是 `/api/...` 路径，不像正式产品页面
- 页面样式是临时 HTML，和 web app 设计语言断裂
- callback endpoint 与浏览器可见页面耦合，后续扩展错误态、loading 态和文案都不干净

## Recommended Approach

采用“前端页与 API callback 分离”的结构：

- `/auth/device`
  - Vue 路由
  - 承担用户可见的输入页、错误提示、提交行为
- `/api/device/verify`
  - 不再输出 HTML
  - 只处理两种请求：
    - `?user_code=...` 时调用 `BeginDeviceVerification` 并 302 到学校 OAuth
    - `?code=...&state=...` 时走现有 OAuth callback 完成绑定

这样保持职责边界清晰：

- 浏览器界面归 web
- OAuth state、cookie、provider 跳转归 API

## User Flow

1. CLI 返回 `verification_url=/auth/device`
2. 用户打开 `/auth/device`
3. 页面引导输入 `user_code`
4. 提交后前端跳到 `/api/device/verify?user_code=...`
5. API 302 到外部 OAuth
6. OAuth 完成后回到 `/api/device/verify?code=...&state=...`
7. API 完成设备绑定并重定向到 `/auth/confirm?...`

## UI Direction

页面视觉参考：

- [device-auth.html](/home/starrydream/ICS2/LabKit/docs/reference/device-auth.html)
- [labkit-design-spec.md](/home/starrydream/ICS2/LabKit/docs/reference/labkit-design-spec.md)

但不机械照搬。实际实现要贴合现有 web shell 和 CSS token：

- 深蓝黑背景 + 微弱网格纹理
- 中央 auth surface，不做低质表单页
- 主视觉重点是 `user_code` 输入栅格
- 文案强调“Authorize this device”
- 错误态、缺参态、提交中态都要有完整反馈

`/auth/confirm` 也同步 polish 到同一套 auth surface，避免一个页面精致、另一个仍像占位页。

## Routing Decisions

- `CreateDeviceAuthorizationRequest` 返回的 `verification_url` 改为 `/auth/device`
- `LABKIT_OAUTH_REDIRECT_URL` 仍保持 `/api/device/verify`
- API router 继续保留 `GET /api/device/verify`
- web router 新增 `GET /auth/device`

这保证：

- 对外 OAuth callback 地址稳定
- CLI 和用户看到的是正常 web 路径

## Error Handling

`/auth/device` 需要显式处理：

- 空输入
- 长度不合法
- 非法字符
- `error=...` 或显式失败 query 参数

`/api/device/verify` 不再渲染错误 HTML。若请求形态不合法，直接返回结构化 HTTP 错误或简单文本错误即可，因为它已经不是产品界面。

## Testing

API:

- router test 断言 `verification_url` 变为 `/auth/device`
- `GET /api/device/verify?user_code=...` 仍会 302 到 provider authorize URL
- `GET /api/device/verify` 不再返回 HTML 表单

Web:

- router test 覆盖 `/auth/device`
- view test 覆盖：
  - query 预填 user code
  - 提交时跳向 `/api/device/verify?user_code=...`
  - 错误提示和按钮状态

## Non-Goals

- 不把 OAuth callback 搬到前端
- 不引入新的后端 JSON API 来取代现有 redirect 流
- 不在这轮扩展 device auth 之外的登录模型
