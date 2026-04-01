# 本地认证与 Admin 鉴权说明

这份文档只描述当前仓库里已经实现的行为，不描述未来计划。

## 1. 当前有哪几种身份进入方式

LabKit 现在有三条不同的身份路径：

1. 学生 CLI / 学生个人 API
   常规路径是设备授权后在本地保存 Ed25519 私钥，之后所有个人操作都走签名鉴权。

2. 浏览器学生会话
   只会在 `/api/device/verify` 的 OAuth 回调成功后创建。它的作用是让网页可以读取“当前学生自己的”历史、设备、公钥等信息。

3. Admin
   现在不是用户登录体系，而是单个全局 Bearer Token。服务端从环境变量 `LABKIT_ADMIN_TOKEN` 读取，前端只是把这个 token 带到 `Authorization: Bearer <token>` 里。

这三条路径当前没有统一到一个“用户中心”里，这是当前实现状态。

## 2. 学生 CLI 是怎么登录的

CLI 登录走设备授权模型：

1. `labkit auth` 本地生成 Ed25519 密钥对。
2. CLI 调 `POST /api/device/authorize`，把公钥发给 API。
3. API 返回：
   - `device_code`
   - `user_code`
   - `verification_url`
   - `expires_at`
4. 用户在浏览器里完成设备确认。
5. CLI 持续调 `POST /api/device/poll` 轮询。
6. 绑定成功后，CLI 把本地私钥和当前 server 的 key metadata 写到本机 keyring。

当前 CLI 配置已经统一切到 TOML：

- 全局配置：`~/.config/labkit/config.toml`
- 项目配置：从当前工作目录向上查找最近的 `.labkit/config.toml`

建议把身份信息和项目默认值分开：

- 全局配置里保留多 server keyring
- 项目配置里保留 `server_url`、`lab`

示例：

```toml
# ~/.config/labkit/config.toml
default_server_url = "http://localhost:8083"

[servers."http://localhost:8083"]
key_path = "/home/user/.config/labkit/id_ed25519"
key_fingerprint = "SHA256:..."
encrypted = false
```

```toml
# .labkit/config.toml
server_url = "http://localhost:8083"
lab = "local-smoke"
```

之后这些命令都走签名请求，不再依赖学校 OAuth token：

- `submit`
- `history`
- `nick`
- `track`
- `keys`
- `revoke`

签名请求头当前已经切到：

- `X-LabKit-Key-Fingerprint`
- `X-LabKit-Timestamp`
- `X-LabKit-Nonce`
- `X-LabKit-Signature`

不再发送 `X-LabKit-Key-ID`。

## 3. 浏览器里的“当前学生”是怎么来的

当前浏览器侧没有独立的用户名密码登录页，也没有单独的 session 登录 API。

浏览器会话只在 OAuth 回调成功后创建：

1. 浏览器访问 `/api/device/verify`
2. 走 OAuth 授权码回调
3. API 在回调处理成功后创建 `labkit_browser_session` Cookie
4. 再重定向到 `/auth/confirm`

这个浏览器 session 的当前实现有两个重要约束：

- 它是进程内内存 session，不落数据库
- API 进程重启后 session 会失效

当前 TTL 是 8 小时。

所以现在浏览器里的“当前学生”能力，更准确地说是“设备绑定完成后的短期辅助会话”，不是完整的正式网页登录体系。

仓库里现在还新增了一个网页登录 ticket 的骨架：

1. CLI 会先用现有 Ed25519 签名身份申请一次性 ticket
2. 服务端返回的目标地址使用 fragment，例如 `/auth/session#ticket=...`，不会把 ticket 放进 query string
3. 浏览器打开 `/auth/session`，页面里的最小 shell 读取 fragment 后同源 `POST` 到 `/auth/session/exchange`
4. 服务端在 `POST` 里消费 ticket、写入 `labkit_browser_session` Cookie
5. 然后先回跳到现有的 `/auth/confirm?mode=web-session`

当前 ticket 申请也会校验 `redirect_path` 必须是站内相对路径，不能是绝对 URL 或 protocol-relative URL。
shell 和兑换响应都会带 `Cache-Control: no-store`，页面脚本会在提交前清掉 fragment，减少本地历史残留。

这条 ticket 流当前还是服务端进程内的临时实现，数据库持久化和一次性消费的正式落地会在后续任务补上。

## 4. Admin 现在怎么鉴权

Admin 现在不是 OAuth，也不是课程组账号体系，而是一个全局 token：

- 服务端环境变量：`LABKIT_ADMIN_TOKEN`
- 前端请求头：`Authorization: Bearer <token>`

本地默认值见 [deploy/.env.example](/home/starrydream/ICS2/LabKit/deploy/.env.example)。

当前前端的 admin 用法是：

1. 打开 `http://localhost:<port>/admin?token=<admin-token>`
2. 前端把 URL 里的 `token` 写进 `sessionStorage`
3. 后续 admin 请求自动带 Bearer Token
4. URL 里的 `token` 会被前端移除

这意味着：

- admin 权限当前是“持有 token 即可”
- 没有课程组成员级别的登录态
- 没有细粒度 RBAC

这是当前仓库的真实状态，不是最终学校 OAuth 接入后的长期方案。

## 5. 没接真实 OAuth 时，怎么做本地认证

在没有学校 OAuth 的情况下，本地开发依赖 dev-only 的快捷绑定接口：

- `POST /api/dev/device/bind`

这个接口只在 `LABKIT_DEV_MODE=true` 时注册。

### 5.1 前提

先确认 [deploy/.env](/home/starrydream/ICS2/LabKit/deploy/.env) 里至少有：

```env
LABKIT_HTTP_PORT=8083
LABKIT_DEV_MODE=true
LABKIT_ADMIN_TOKEN=dev-admin-token
```

然后启动本地栈：

```bash
bash scripts/dev-up.sh
```

### 5.2 CLI 本地认证步骤

终端 A：

```bash
go run ./apps/cli/cmd/labkit --server-url http://localhost:8083 auth --no-encrypt
```

终端 B，取最近一次设备授权产生的 `device_code`：

```bash
docker compose -f deploy/docker-compose.yml --env-file deploy/.env exec -T postgres \
  psql -U labkit -d labkit -Atc \
  "select device_code from device_auth_requests order by created_at desc limit 1;"
```

终端 B，用 dev bind 接口直接批准这个设备：

```bash
curl -X POST http://localhost:8083/api/dev/device/bind \
  -H 'Content-Type: application/json' \
  -d '{"device_code":"<device_code>","student_id":"2026001","device_name":"local-dev"}'
```

成功后，终端 A 里的 `labkit auth` 会结束，并把当前 server 的 keyring entry 写好。

补充说明：

- 同一台机器对不同 `server_url` 默认使用不同本地 key
- 对同一个 `server_url` 再次运行 `labkit auth` 时，默认复用已有 key
- 如果你明确要换掉当前 server 的设备身份，用 `labkit auth --rotate-key`
- 在交互式终端里，`labkit auth` 会询问是否用 passphrase 加密私钥
- 在脚本或非交互环境里，必须显式传 `--encrypt` 或 `--no-encrypt`

如果你已经在仓库里放了 `.labkit/config.toml`，后续命令通常可以直接写成：

```bash
go run ./apps/cli/cmd/labkit submit submission.json
go run ./apps/cli/cmd/labkit board
go run ./apps/cli/cmd/labkit history
```

### 5.3 这条本地捷径的限制

这条路径只是为了本地开发，不是正式部署方案。它有几个明显限制：

- 需要手工查 `device_code`
- 不经过真实 OAuth
- 学号完全由调用方自己填
- 只能在 `LABKIT_DEV_MODE=true` 的开发环境使用

## 6. 当前推荐的本地使用方式

如果你只是想在本机把整条链路跑通，建议按下面顺序：

1. 用 `http://localhost:8083/admin?token=dev-admin-token` 进入 admin
2. 注册一个测试 lab
3. 用本文档里的 dev bind 完成一次 `labkit auth`
4. 用 CLI 跑一次 `submit`
5. 回到 admin queue 页面看 job
6. 回到 leaderboard 页面看结果

关于 lab manifest、evaluator 镜像和输出协议，见 [lab-authoring.md](/home/starrydream/ICS2/LabKit/docs/reference/lab-authoring.md)。
