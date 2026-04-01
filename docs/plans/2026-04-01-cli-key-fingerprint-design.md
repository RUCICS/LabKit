# CLI Key Fingerprint、每服务器独立密钥与可选私钥加密设计

日期：2026-04-01

## 背景

当前 LabKit CLI 的个人认证协议和本地密钥管理有三个问题：

1. CLI 协议暴露 `key_id`
   - `key_id` 是服务端数据库主键
   - 跨服务器不稳定
   - 把内部实现细节泄漏进了公开协议

2. 全局配置当前是扁平结构
   - 只能表达“一个当前服务器”的身份
   - 如果对多个服务器分别 `auth`，后一次会覆盖前一次

3. 本地私钥当前只支持明文落盘
   - `0600` 权限对开发场景可接受
   - 但没有给更谨慎的用户提供 passphrase 保护选项

## 目标

这一轮解决四件事：

1. 用稳定的公钥 fingerprint 替代 `key_id` 作为 CLI 协议里的密钥标识
2. 让全局配置支持按 `server_url` 存多套身份材料
3. 默认每个服务器自动生成独立密钥，而不是跨服务器共用一把 key
4. 在 `labkit auth` 时提供可选私钥加密，交互风格类似 SSH

## 方案选择

### A. 保留 `key_id`，只修多服务器配置

优点：

- 服务端改动小

缺点：

- 协议仍然依赖数据库主键
- 根问题没解决

不选。

### B. 改用公钥 fingerprint，但所有服务器共用同一把 key

优点：

- 协议更干净
- 配置看起来更简单

缺点：

- 不同 trust domain 共用一把私钥，隔离太差
- 任一服务器的泄露或撤销都会影响其他服务器

不选。

### C. 改用公钥 fingerprint，并按服务器独立管理密钥

行为：

- 每个 `server_url` 自动拥有独立 key
- CLI 用本地私钥推导公钥，再计算 fingerprint
- 请求头发送 fingerprint，而不是 `key_id`
- 全局配置按服务器分桶存身份材料

优点：

- 协议不再依赖数据库内部 ID
- 多服务器身份隔离清晰
- 更符合长期演进方向

结论：采用这个方案。

## 协议设计

CLI 认证头从：

- `X-LabKit-Key-ID`
- `X-LabKit-Timestamp`
- `X-LabKit-Nonce`
- `X-LabKit-Signature`

调整为：

- `X-LabKit-Key-Fingerprint`
- `X-LabKit-Timestamp`
- `X-LabKit-Nonce`
- `X-LabKit-Signature`

### fingerprint 格式

采用 SSH 风格表示：

- 输入：Ed25519 公钥原始 bytes
- 算法：`SHA-256`
- 编码：`base64.RawStdEncoding`
- 头值格式：`SHA256:<base64>`

例如：

```text
SHA256:AbCdEf...
```

服务端收到 fingerprint 后：

1. 根据 fingerprint 找到匹配的 `user_keys.public_key`
2. 解析公钥
3. 按现有 payload / nonce / timestamp 规则验签

数据库里的 `submissions.key_id`、`web_session_tickets.key_id` 这类内部字段可以保留。
它们属于服务端内部外键和审计链，不必暴露给 CLI 协议。

## 全局配置设计

全局配置改成多服务器结构。

示例：

```toml
default_server_url = "http://localhost:8083"

[servers."http://localhost:8083"]
key_path = "/home/user/.config/labkit/keys/localhost_8083_ed25519"
key_fingerprint = "SHA256:..."
encrypted = false

[servers."https://labkit.example.edu"]
key_path = "/home/user/.config/labkit/keys/labkit.example.edu_ed25519"
key_fingerprint = "SHA256:..."
encrypted = true
```

规则：

- `default_server_url` 是可选默认值
- `servers.<origin>` 以标准化后的 server origin 为 key
- 每个服务器单独存：
  - `key_path`
  - `key_fingerprint`
  - `encrypted`

项目配置 `.labkit/config.toml` 继续只负责：

- `server_url`
- `lab`

## 每服务器独立密钥策略

默认策略定为：

- 同一台机器、不同 `server_url` 自动生成不同 key

原因：

- 身份隔离更好
- 某个服务器撤销或轮换 key 不影响其他服务器
- 更符合“不同 trust domain 不共钥”的常见实践

## auth 命令行为

`labkit auth` 的行为改为：

1. 先解析最终 `server_url`
2. 查全局配置里这个 server 是否已有 key
3. 如果没有：
   - 生成新 key
   - 开头明确提示“正在为该服务器创建新的本地 key”
4. 如果已有：
   - 默认复用已有 key
   - 开头明确提示“正在复用现有本地 key”
   - 提示里显示 fingerprint
5. 若用户显式传 `--rotate-key`
   - 为当前 server 重新生成新 key
   - 更新这一桶配置

也就是说，同一服务器重复 `auth` 默认不换 key，只是复用已有设备身份继续绑定。

## 私钥加密设计

当前私钥明文文件仍然允许存在，但加密成为可选能力。

### 交互模式

在 TTY 中执行 `labkit auth` 时：

1. 如果当前 server 还没有 key，或指定了 `--rotate-key`
2. CLI 在生成并写盘前询问：
   - 是否用 passphrase 加密私钥

如果用户选择加密：

- 输入两次 passphrase
- 用加密 PEM 私钥文件写盘

如果用户选择不加密：

- 保持当前明文 PKCS#8 PEM
- 文件权限仍为 `0600`

### 非交互模式

为了避免脚本或测试被 prompt 卡住，增加显式 flag：

- `--encrypt`
- `--no-encrypt`

规则：

- 在非交互环境中必须靠 flag 决定是否加密
- 在交互环境中，flag 优先于 prompt

### 运行时解密

读取私钥时：

- 如果是明文 PEM，直接读取
- 如果是加密 PEM，提示输入 passphrase 后解密

这一轮不做：

- passphrase 缓存
- agent
- 系统 keychain
- 硬件 token

## 用户可见输出

`labkit auth` 开头统一打印当前 key 状态：

- 复用已有 key 时，提示：
  - server
  - fingerprint
  - “reusing existing key”
- 创建新 key 时，提示：
  - server
  - fingerprint（生成后打印）
  - “created new key”

如果使用 `--rotate-key`，也要明确提示当前是在轮换 key。

## 边界

这轮不做：

- 把服务端内部所有 `key_id` 都删掉
- 为已存在数据做复杂历史回填
- 系统 keychain / 硬件密钥
- 多 profile / 多用户
- 自动把明文私钥强制迁移成加密私钥

## 验证标准

完成后应满足：

- CLI 协议不再依赖 `key_id`
- 同一台机器可对多个服务器独立持有身份，不互相覆盖
- 同一服务器重复 `auth` 默认复用已有 key
- 用户可显式 `--rotate-key`
- `labkit auth` 能在交互模式里询问是否加密私钥
- 非交互模式可通过 flag 明确控制是否加密
- 明文私钥仍可用，加密私钥也可成功解密并签名
