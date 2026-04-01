# CLI 网页续登录与输出体验设计

日期：2026-04-01

## 背景

当前 LabKit 的身份模型是：

- `labkit auth` 负责首次设备绑定
- CLI 后续所有个人操作都靠本地 Ed25519 私钥签名
- 浏览器 session 只在 OAuth 回调成功时创建

这会导致一个明确的 UX 缺口：

- 学生已经在 CLI 里完成过 `auth`
- 但网页 session 过期后，浏览器没有办法靠“已有设备身份”重新登录

同时，CLI 现在的输出层仍然偏原始：

- 主要依赖 `fmt.Fprintf`
- 表格和状态输出可读性一般
- 缺少更明确的步骤感、状态感和视觉秩序

## 目标

解决两个问题：

1. 提供一个不重新跑完整 OAuth 设备绑定的“网页续登录”命令
2. 给 CLI 引入更产品化的输出层，但不把命令式工具强行做成全屏 TUI

## 方案选择

### A. 重新跑 `auth`

优点：

- 复用现有链路

缺点：

- 明显 UX 过重
- 语义不对。用户不是在“重新绑定设备”，只是想重新拿浏览器 session

不选。

### B. 网页自己发起独立登录

优点：

- 浏览器视角直觉

缺点：

- 会长出一套独立于 CLI 的网页登录模型
- 和当前“CLI 是主身份载体”的产品方向冲突

不选。

### C. `labkit web` 用已有签名身份换一次性浏览器 ticket

流程：

1. CLI 读取本地私钥和 `key_id`
2. CLI 以签名请求调用新的 API，申请一次性网页登录 ticket
3. 服务端返回短 TTL、一次性消费的 `ticket`
4. CLI 自动打开浏览器到带 `ticket` 的 URL
5. 服务端消费 `ticket`，创建浏览器 session cookie
6. 浏览器跳转到目标页面

优点：

- 不重新跑 OAuth
- 不重新绑设备
- 身份源仍然是 Ed25519 设备密钥
- 与当前架构最一致

结论：采用这个方案。

## 命令设计

命令名定为：

- `labkit auth`
  首次绑定设备身份
- `labkit web`
  用已绑定设备身份恢复或打开浏览器侧登录态

`labkit web` 的行为：

- 默认打开首页或 `/profile`
- 支持可选目标路径，例如：
  - `labkit web`
  - `labkit web /profile`
  - `labkit web /labs/local-smoke/board`

CLI 内部仍然是“签名换 ticket”，只是对用户暴露成一个简短命令。

## API 设计

新增两个端点：

1. `POST /api/web/session-ticket`
   - 需要 CLI 签名鉴权
   - 请求体：
     - `redirect_path`
   - 返回：
     - `ticket`
     - `expires_at`
     - `redirect_url`

2. `GET /auth/session`
   - 查询参数：
     - `ticket`
   - 服务端校验通过后：
     - 创建 `labkit_browser_session`
     - 标记 ticket 已消费
     - 跳转到目标路径

ticket 设计要求：

- 一次性
- 短 TTL，例如 60 秒
- 绑定 user_id / key_id / redirect_path
- 服务端重启后不丢失，所以应存数据库，而不是内存 map

## CLI 输出层设计

库选择：

- 主样式层：`Lip Gloss`
- 可选 Markdown 渲染：`glamour`

不优先用 `PTerm`，因为它更像一套现成终端组件系统，风格控制不如 `Lip Gloss` 自由，容易带出较强库味。

CLI 输出层的目标不是“炫”，而是：

- 结构清楚
- 有状态感
- 可扫描
- 保留脚本友好能力

### 输出模式

建议保留三种：

- 默认：彩色、结构化、人类友好
- `--plain`：去掉样式，但保留结构
- `--json`：机器可消费

### 首批命令优化

按价值排序：

1. `auth`
   - 卡片式显示 verification URL / user code
   - 轮询时显示状态
   - 成功后显示绑定完成摘要

2. `web`
   - 明确显示“正在打开浏览器”
   - 失败时给出 fallback URL

3. `submit`
   - 上传中状态
   - 成功后显示 submission ID / status / 下一步建议

4. `board`
   - 更紧凑的 mono table
   - 当前 metric 高亮

5. `history`
   - verdict 颜色
   - 时间列和 message 更易读

6. `keys`
   - 设备清单化

## 边界

这轮不做：

- 全屏 TUI
- 浏览器端独立登录体系
- Admin OAuth / RBAC
- WebSocket 或实时终端交互

## 验证标准

完成后应满足：

- 已完成 `auth` 的用户可以仅通过 `labkit web` 恢复浏览器 session
- 网页 session 过期后，不必重跑完整 OAuth 设备绑定
- CLI 默认输出比当前明显更可读
- 仍然保留 `--plain` 或 `--json` 这类脚本友好出口
