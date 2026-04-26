# LabKit CLI 使用说明

这份文档只描述当前仓库里已经实现的 CLI 行为，不描述未来计划。

LabKit 的命令行工具统一叫 `labkit`。它主要解决两类问题：

- 在本地把当前设备绑定到你的课程账号
- 围绕某个 lab 完成提交、看榜、查历史、改昵称、切换赛道等日常操作

如果你只是想先跑起来，可以先看下面这段最短流程：

```bash
# 查看所有命令
go run ./apps/cli/cmd/labkit --help

# 首次认证
go run ./apps/cli/cmd/labkit --server-url http://localhost:8083 auth --no-encrypt

# 进入某个 lab 后查看排行榜
go run ./apps/cli/cmd/labkit --server-url http://localhost:8083 --lab local-smoke board

# 提交文件
go run ./apps/cli/cmd/labkit --server-url http://localhost:8083 --lab local-smoke submit submission.json
```

如果你已经在项目根目录写好了 `.labkit/config.toml`，很多命令就可以省掉 `--server-url` 和 `--lab`。

## 1. CLI 能做什么

当前 `labkit` 提供这些命令：

- `auth`：把当前机器绑定到你的账号
- `keys`：查看当前 server 上已绑定的设备密钥
- `revoke <key-id>`：撤销某个已绑定密钥
- `web [path]`：用 CLI 身份打开网页端会话
- `board`：查看排行榜
- `submit <files...>`：提交作业文件
- `history [submission-id]`：查看提交历史，或查看某次提交详情
- `nick <name>`：修改排行榜昵称
- `track <metric_id>`：选择当前赛道
- `completion`：生成 shell 自动补全脚本

## 2. 运行方式

开发期最直接的方式是：

```bash
go run ./apps/cli/cmd/labkit --help
```

如果你要分发可执行文件，可以先构建：

```bash
bash scripts/build-cli.sh
```

也可以只构建指定平台：

```bash
bash scripts/build-cli.sh linux/amd64 darwin/arm64
```

## 3. 配置文件和优先级

LabKit CLI 当前使用 TOML 配置。建议把“身份信息”和“项目默认值”分开管理。

### 3.1 全局配置

全局配置默认在：

```text
~/.config/labkit/config.toml
```

它主要保存：

- 默认 server
- 按 server 维护的本地密钥路径
- 密钥指纹
- 私钥是否加密

典型示例：

```toml
# ~/.config/labkit/config.toml
default_server_url = "http://localhost:8083"

[servers."http://localhost:8083"]
key_path = "/home/user/.config/labkit/keys/localhost_8083_ed25519"
key_fingerprint = "SHA256:..."
encrypted = false
```

### 3.2 项目配置

项目配置文件是：

```text
.labkit/config.toml
```

CLI 会从当前目录开始，向上查找最近的 `.labkit/config.toml`。这个文件建议只放项目级默认值，例如：

- `server_url`
- `lab`

示例：

```toml
# .labkit/config.toml
server_url = "http://localhost:8083"
lab = "local-smoke"
```

有了它之后，你在这个仓库或它的子目录里就可以直接运行：

```bash
go run ./apps/cli/cmd/labkit board
go run ./apps/cli/cmd/labkit submit submission.json
go run ./apps/cli/cmd/labkit history
```

### 3.3 环境变量和命令行参数

当前最常用的覆盖方式有两个：

- `LABKIT_SERVER_URL`：设置默认 server
- `LABKIT_CONFIG_DIR`：改掉全局配置目录

另外，根命令也支持三个全局参数：

- `--config-dir`：指定配置目录
- `--server-url`：指定 server
- `--lab`：指定当前 lab

### 3.4 覆盖优先级

可以这样理解当前行为：

- `server_url` 的优先级是：`--server-url` > `LABKIT_SERVER_URL` > `.labkit/config.toml` > 全局 `default_server_url` > 内建默认值 `http://localhost:8080`
- `lab` 的优先级是：`--lab` > `.labkit/config.toml`
- 身份信息始终来自全局配置，不从项目配置里读取

这意味着：

- 你可以在仓库里固定某个 lab 的默认 `server_url` 和 `lab`
- 你也可以临时用 `--server-url` 或 `--lab` 覆盖它
- `keys`、`revoke`、`web`、需要签名的 `submit` / `history` / `nick` / `track` 依赖的仍然是全局身份状态

## 4. 推荐使用流程

第一次接入某个 lab 时，建议按下面的顺序来：

1. 在项目根目录写 `.labkit/config.toml`
2. 运行 `labkit auth` 完成设备绑定
3. 先用 `labkit board` 看看当前榜单和 metric
4. 按 manifest 要求准备好提交文件
5. 用 `labkit submit <files...>` 发起提交
6. 用 `labkit history` 回看结果，必要时再用 `labkit history <submission-id>` 看详情

如果只是日常使用，通常就是三条命令轮换：

```bash
labkit board
labkit submit <files...>
labkit history
```

## 5. 命令详解

### 5.1 `labkit auth`

用途：把当前机器绑定到你的账号，并为当前 server 准备本地 Ed25519 私钥。

基本用法：

```bash
labkit auth
labkit auth --rotate-key
labkit auth --encrypt
labkit auth --no-encrypt
```

它的实际行为是：

1. 解析当前 server URL
2. 检查这个 server 是否已经有可复用的本地私钥
3. 如无可用密钥，则生成新的 Ed25519 密钥对
4. 调 `POST /api/device/authorize` 发起设备授权
5. 在终端显示浏览器访问地址和用户验证码
6. 持续轮询，直到服务端确认授权完成
7. 把这个 server 的 keyring 信息写回全局配置

需要特别注意的点：

- 对同一个 `server_url` 再次运行 `auth` 时，默认会复用已有密钥
- 只有传 `--rotate-key` 才会主动生成新密钥
- 交互式终端里，如果你没显式传 `--encrypt` 或 `--no-encrypt`，CLI 会询问是否给私钥加口令
- 非交互环境里，必须显式传 `--encrypt` 或 `--no-encrypt`
- 如果私钥已经加密，后续需要签名的命令会要求你输入口令

本地开发没有真实 OAuth 时，可以参考 [本地认证与 Admin 鉴权说明](/home/starrydream/ICS2/LabKit/docs/reference/local-auth.md) 里 dev bind 的做法。

### 5.2 `labkit keys`

用途：列出当前 server 上你已经绑定的设备密钥。

基本用法：

```bash
labkit keys
```

这个命令需要先完成 `auth`。输出里会显示：

- key id
- device name
- 创建时间

如果你看到 `key id is required; run auth first`，说明当前 server 还没有可用身份。

### 5.3 `labkit revoke <key-id>`

用途：撤销某个已绑定设备。

基本用法：

```bash
labkit revoke 3
```

这个命令会向服务端发送签名请求，撤销指定的 key id。它不会自动删除你本地磁盘上的私钥文件；它做的是“让这个密钥不再被服务端接受”。

### 5.4 `labkit web [path]`

用途：用 CLI 当前身份直接打开网页端会话。

基本用法：

```bash
labkit web
labkit web /profile
labkit web /labs/local-smoke
```

默认跳转路径是 `/profile`。

这个命令有两个约束：

- 必须先 `auth`
- `path` 必须是站内相对路径，必须以 `/` 开头，不能写成完整 URL

CLI 会先向服务端申请一次性网页登录 ticket，再尝试自动打开浏览器。如果自动打开失败，它会把 URL 打印出来，供你手动访问。

### 5.5 `labkit board`

用途：查看当前 lab 的排行榜。

基本用法：

```bash
labkit board
labkit board --by runtime_ms
```

默认情况下，排行榜按 lab 当前默认 metric 排序；如果你传了 `--by`，就按指定 metric 查看。

这个命令有两个使用模式：

- 未认证也能访问公开排行榜
- 已认证时会优先走签名请求，服务端可以返回“当前用户所在行”以及个人 quota 信息

如果项目里没配置 lab，运行时会报：

```text
lab id is required; pass --lab or set lab in .labkit/config.toml
```

### 5.6 `labkit submit <files...>`

用途：提交当前 lab 要求的文件。

基本用法：

```bash
labkit submit main.c README.md
labkit submit submission.json
labkit submit main.c README.md --detach
labkit submit main.c README.md --no-wait
```

这是 CLI 里最重要的命令。它的行为可以分成几步：

1. 读取 lab manifest
2. 按 manifest 的 `[submit].files` 严格校验你传入的文件名
3. 本地打包并计算提交内容哈希
4. 发起签名 precheck，拿到 quota 信息和最近一次提交的 hash 提示
5. 正式上传文件
6. 默认持续轮询，直到这次提交进入终态
7. 输出结果块、分数和 quota 摘要

文件校验是严格的：

- 缺文件会直接报错
- 多传文件会直接报错
- 同名重复文件会直接报错
- 校验依据是文件名，不是相对路径

关于等待行为：

- 默认会等待评测完成
- `--detach` 和 `--no-wait` 都表示“提交后立刻退出，不等结果”
- 如果你在等待过程中按 `Ctrl+C`，提交仍会继续在服务端运行；之后可以用 `labkit history` 或 `labkit board` 再看

关于重复提交提示：

- 如果本次归档 hash 和你最近一次提交完全一致，交互式终端会给你一个软确认
- 直接按回车表示继续提交
- 输入 `n` 或 `no` 表示取消
- 非交互环境不会阻塞，只会打印一条 warning 然后继续提交

### 5.7 `labkit history [submission-id]`

用途：查看当前 lab 的提交历史，或者查看单次提交详情。

基本用法：

```bash
labkit history
labkit history sub_123456
```

不带参数时，会列出你在当前 lab 下的历史提交记录；带 `submission-id` 时，会直接拉这次提交的详情，包括：

- 当前状态
- verdict
- 运行时长
- 分数
- 详细说明

这个命令必须先 `auth`。

### 5.8 `labkit nick <name>`

用途：修改你在当前 lab 排行榜上的昵称。

基本用法：

```bash
labkit nick alice
```

这个命令必须先 `auth`，也必须能确定当前 lab。

### 5.9 `labkit track <metric_id>`

用途：声明你当前主打的赛道。

基本用法：

```bash
labkit track throughput
labkit track runtime_ms
```

它有两个前提：

- 当前 lab 的 manifest 里 `[board].pick = true`
- 你传入的 `metric_id` 必须是 manifest 里已声明的 metric

如果 lab 没开启赛道选择，CLI 会直接报 `track selection is disabled`。

### 5.10 `labkit completion`

用途：生成 shell 自动补全脚本。

例如：

```bash
labkit completion bash
labkit completion zsh
```

这是 Cobra 自带的标准命令，适合你把 `labkit` 装进本机 PATH 之后再使用。

## 6. 常见报错和处理方式

### 6.1 `server URL is required`

说明当前命令无法确定要连哪个 server。常见解决方式：

- 补 `--server-url`
- 设置 `LABKIT_SERVER_URL`
- 在 `.labkit/config.toml` 里写 `server_url`
- 先跑一次 `labkit auth`，让全局配置记住默认 server

### 6.2 `lab id is required; pass --lab or set lab in .labkit/config.toml`

说明当前命令需要知道你在操作哪个 lab。解决方式通常是：

- 传 `--lab <lab-id>`
- 在项目根目录写 `.labkit/config.toml`

### 6.3 `key id is required; run auth first`

说明这是一个必须签名的命令，但当前 server 还没有可用身份。先运行：

```bash
labkit auth
```

### 6.4 `private key is encrypted; rerun in an interactive terminal`

说明你的私钥已经加密，但当前命令运行在非交互环境，CLI 没法弹出口令提示。改成在交互式终端里运行即可。

### 6.5 `redirect path must be a site-relative path`

说明你给 `labkit web` 传的路径不合法。正确写法类似：

```bash
labkit web /profile
labkit web /labs/local-smoke
```

不要写成：

```bash
labkit web https://example.com/profile
```

## 7. 一套更顺手的日常配置

如果你经常在某个 lab 仓库里工作，最省事的方式是：

先在仓库根目录放一个 `.labkit/config.toml`：

```toml
server_url = "http://localhost:8083"
lab = "local-smoke"
```

然后只做一次认证：

```bash
go run ./apps/cli/cmd/labkit auth --no-encrypt
```

之后日常就可以直接写：

```bash
go run ./apps/cli/cmd/labkit board
go run ./apps/cli/cmd/labkit submit submission.json
go run ./apps/cli/cmd/labkit history
go run ./apps/cli/cmd/labkit web
```

这也是当前仓库最推荐的 CLI 使用方式。
