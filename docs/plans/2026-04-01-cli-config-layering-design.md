# CLI 本地/全局配置分层与 TOML 迁移设计

日期：2026-04-01

## 背景

当前 LabKit CLI 的配置模型过于单薄：

- 全局只读取 `~/.config/labkit/config.json`
- `server_url` 能进全局配置
- `lab` 不能进配置，只能每次传 `--lab`
- 项目目录下没有任何本地配置发现机制

这导致两个明确问题：

1. 在同一个仓库里反复执行 `labkit --server-url ... --lab ...` 很啰嗦
2. CLI 没有“项目上下文”，无法像 Git / npm / eslint 一样在子目录里继承仓库级默认配置

同时，当前配置格式是 JSON。对手写编辑来说，TOML 更适合这个项目已有的风格，也和仓库里 manifest 的使用方式更一致。

## 目标

这一轮解决三个问题：

1. 支持项目级本地配置
2. 让 `lab` 和 `server_url` 都能通过配置提供默认值
3. 把 CLI 配置格式从 JSON 全量切到 TOML

## 方案选择

### A. 只支持全局 TOML 配置

优点：

- 改动最小

缺点：

- 不能解决“仓库里每次都要传 `--lab` / `--server-url`”的问题

不选。

### B. 只读取当前工作目录下的本地配置

优点：

- 实现简单
- 语义直观

缺点：

- 在子目录里执行会失效
- 不符合多数项目工具的使用预期

不选。

### C. 引入全局配置 + 向上查找的项目配置，并统一切到 TOML

行为：

- 全局配置：`~/.config/labkit/config.toml`
- 项目配置：从当前目录开始，向上查找最近的 `.labkit/config.toml`
- CLI 参数和环境变量继续保留，并覆盖配置值

优点：

- 最接近 Git / eslint / prettier 等工具的体验
- 兼顾个人身份配置和项目上下文配置
- 用户在仓库任意子目录运行都能拿到一致默认值

结论：采用这个方案。

## 配置模型

配置来源分两层：

1. 全局配置
2. 项目本地配置

字段职责如下：

- `server_url`
  - 可放在全局配置
  - 可放在项目配置
- `lab`
  - 可放在全局配置
  - 可放在项目配置
- `key_id`
  - 只放在全局配置
- `key_path`
  - 只放在全局配置

这样边界比较干净：

- 全局配置负责“这台机器的身份”
- 项目配置负责“这个仓库默认连哪个服务、默认操作哪个 lab”

项目配置示例：

```toml
server_url = "http://localhost:8083"
lab = "local-smoke"
```

全局配置示例：

```toml
server_url = "https://labkit.example.edu"
lab = "colab-2026-p2"
key_id = 12
key_path = "/home/user/.config/labkit/id_ed25519"
```

## 优先级规则

配置覆盖顺序定为：

`CLI flags > environment variables > local project config > global config > built-in defaults`

具体规则：

- `--server-url` 优先级最高
- `LABKIT_SERVER_URL` 次之
- 如果两者都没有，则先读项目配置，再读全局配置
- `--lab` 优先级最高
- `lab` 没有环境变量层，直接走：
  - CLI flag
  - local config
  - global config
  - 否则报错

## 本地配置发现

本地配置查找算法：

1. 从当前工作目录开始
2. 检查 `<dir>/.labkit/config.toml`
3. 如果存在则使用并停止查找
4. 否则移动到父目录继续
5. 到达文件系统根目录后停止

不要求当前目录必须是 Git 仓库。这样行为更通用，也更容易解释。

## 命令行为变化

实现后，下面这些命令都应受益于本地配置：

- `submit`
- `board`
- `history`
- `nick`
- `track`
- `web`

表现为：

- 在仓库任意子目录执行时，若存在最近的 `.labkit/config.toml`，命令会自动继承 `lab` 和 `server_url`
- 仍然允许通过 `--lab` / `--server-url` 临时覆盖

`auth` 仍然只写全局配置，因为它管理的是设备身份，不是项目上下文。

## 迁移策略

用户已经明确要求全量切换，不兼容旧 JSON。

因此行为定为：

- CLI 只读取 `config.toml`
- CLI 不再读取任何 `config.json`
- `auth` 和其他写配置逻辑只写 TOML
- 文档、测试、示例全部切换为 TOML

如果用户仍然保留旧 JSON，CLI 视为“无配置”。

## 边界

这轮不做：

- 自动把旧 `config.json` 迁移成 `config.toml`
- `labkit config set` / `labkit config get` 子命令
- 项目配置里的身份字段（`key_id`、`key_path`）
- 多工作区或 profile 机制

## 验证标准

完成后应满足：

- 在带 `.labkit/config.toml` 的仓库任意子目录运行 `labkit board` 时，不必再传 `--lab` 和 `--server-url`
- `--lab` 和 `--server-url` 仍然可以覆盖本地配置
- `auth` 会写全局 `config.toml`
- CLI 不再依赖任何 `config.json`
- 相关文档和测试都反映新的 TOML 配置模型
