# CLI 交互与终端体验重设计

日期：2026-04-01

## 背景

当前 LabKit CLI 已经具备基本能力，但交互模型仍然偏“脚本式”：

- `submit` 提交后立刻返回，用户还要自己再跑 `history` 查结果
- `auth`、`web`、`history`、`board` 主要是“打一条请求，吐一段文本”
- 终端输出虽然已经开始引入 `Lip Gloss`，但目前更多是给原有文本套壳
- 用户感受到的是“换了个皮”，而不是“更好的开发者工具”

这和用户期望的开发者体验有明显差距。参照物更接近 `uv`、`cargo` 这类工具：

- 有任务感
- 有阶段反馈
- 有合理的等待与完成语义
- 最终结果不是一坨原始文本，而是一份可扫描的终端报告

## 目标

这轮重设计要解决的是 CLI 的交互模型，而不只是视觉样式：

1. `submit` 默认等待到最终状态，而不是提交后立即退出
2. `submit` 在等待期间持续反馈状态变化
3. `submit` 完成后输出紧凑但信息足够的结果报告
4. `auth`、`history`、`board` 也改成更像正式开发者工具的交互与排版
5. `keys`、`web`、`nick`、`track` 做低成本一致化，不抢主线

## 方案比较

### 方案 A：继续做输出美化

做法：

- 保留当前命令行为
- 继续补 `Lip Gloss` 样式
- 用卡片、标题、颜色强化已有输出

优点：

- 成本最低
- 风险小

缺点：

- 核心交互问题不解决
- `submit` 仍然像“提交了就没了”
- 最终效果仍然会被用户视为“换皮”

不采用。

### 方案 B：任务驱动型 CLI

做法：

- 把 `submit`、`auth`、`web` 这类动作重构成明确的任务流程
- 在执行过程中给阶段更新、轮询状态、完成总结
- `history`、`board`、`keys` 则重构信息组织方式，让它们像终端工具输出，而不是裸表格

优点：

- 真正解决用户体验问题
- 最贴近 `uv` / `cargo` 一类开发者工具
- 可以在保留命令式 CLI 的前提下显著提升产品感

缺点：

- 需要补一层状态流和 UI 组件抽象
- `submit` 可能还会牵引少量服务端查询能力

结论：采用这个方案。

### 方案 C：做半交互式全屏 TUI

做法：

- 用 `Bubble Tea` 一类框架改成更多全屏交互
- `history` / `board` / `submit` 都做终端内导航

优点：

- 理论上可做得很强

缺点：

- 当前明显过重
- 实现和维护成本高
- 不适合现在这轮以命令式工具为主的目标

不采用。

## 总体设计

CLI 分成两类命令：

- **任务型命令**
  - `submit`
  - `auth`
  - `web`
- **信息型命令**
  - `history`
  - `board`
  - `keys`

这两类命令用不同的终端组织方式。

### 任务型命令

任务型命令要有：

- 明确的阶段流
- 当前状态
- 必要时的加载动画
- 完成时的结果卡
- 出错时的 detail block

用户应该能一眼知道：

- 程序正在做什么
- 现在到哪一步
- 最后是否成功
- 如果失败，为什么失败

### 信息型命令

信息型命令不做全屏交互，但要改信息架构：

- 重点字段突出
- 长字段裁切
- 状态和时间有层级
- 列表和表格不是单纯 `tabwriter` 原样吐出

## `submit` 设计

这是本轮最重要的改动。

### 默认行为

`labkit submit ...` 默认执行到最终状态为止：

1. 本地校验文件
2. 拉取 manifest
3. 打包 submission
4. 签名请求
5. 上传
6. 拿到 `submission_id`
7. 进入轮询等待
8. 得到最终 `status / verdict`
9. 输出结果报告

用户不应再手动“提交后再查一次”。

### 轮询行为

提交成功进入等待后，CLI 只在状态变化时刷新：

- `queued`
- `running`
- `scored`
- `build_failed`
- `rejected`
- `error`

终端输出应当是“同一个任务在推进”，而不是不断往下刷屏。

### 默认结果报告

提交结束后默认给一份折中的结果卡：

- `submission id`
- 最终 `status / verdict`
- `elapsed`
- 如果 `scored`
  - 主 metric / 总分
  - 所有 metric 的紧凑列表
- 如果 `build_failed / rejected / error`
  - 自动展开 `message`
  - 自动展开失败 `detail`

如果将来需要更长输出，再加 `--verbose`，但不是这轮前提。

### 控制选项

默认等待到完成。

额外提供：

- `--detach`
- `--no-wait`

显式指定时才提交后立刻退出。

`Ctrl+C` 的语义：

- 不取消服务端任务
- 只停止本地等待
- 退出前提示用户稍后用 `history` 查看

## `auth` 设计

`auth` 不再只是打印一行 URL。

它要变成完整流程：

1. `generate keypair`
2. `request device authorization`
3. `open browser / show verification code`
4. `waiting for approval`
5. `authorized`

显示内容：

- verification URL
- user code
- 当前轮询状态
- 绑定完成后的 `student_id / key_id`

如果自动打开浏览器失败，要给出明确 fallback，而不是只剩一行原始文本。

## `history` 设计

`history` 默认输出从“原始表格”改为“最近提交流”。

每条记录突出：

- submission id 短前缀
- `status / verdict`
- 时间
- 核心分数或失败原因

最新一条应更易识别。

这轮先不做复杂交互，但后续可以自然扩展：

- `--limit`
- `--detail <submission-id>`
- `--watch`

## `board` 设计

`board` 默认输出应像一张排行榜快照，而不是原始 tabwriter。

需要突出：

- 当前 lab
- 当前 metric
- top rows
- 排名和主 metric
- 如果有 track，则清楚显示

保留可复制、可 grep 的终端文本特性，但要让信息重心更清楚。

## 顺手优化项

这些命令不是本轮主线，但可以低成本统一：

1. `keys`
   - 设备清单化
2. `web`
   - 与新的任务型风格统一
3. `nick`
   - 更干净的成功反馈
4. `track`
   - 更干净的成功反馈

## 组件层设计

不做全屏 TUI，只补一个小而稳定的终端 UI 组件层。

建议目录：

- `apps/cli/internal/ui/theme.go`
- `apps/cli/internal/ui/task.go`
- `apps/cli/internal/ui/card.go`
- `apps/cli/internal/ui/table.go`
- `apps/cli/internal/ui/detail.go`

核心组件：

### 1. Task Flow

用于：

- `submit`
- `auth`
- `web`

负责：

- 步骤文案
- 状态 badge
- spinner / waiting
- 状态切换输出

### 2. Result Card

用于：

- submit 最终结果
- auth 完成
- web 成功 / fallback

### 3. Compact Table / List

用于：

- `board`
- `history`
- `keys`

重点是排版、列权重、裁切，不是靠大边框装饰。

### 4. Detail Block

用于：

- submit 失败 detail
- history 某条失败详情

## 状态语义

CLI 内部统一状态词表：

- `queued`
- `running`
- `scored`
- `build_failed`
- `rejected`
- `error`

每个状态要有稳定颜色和文案，不要每个命令各自解释。

## 实现顺序

按基础设施优先，而不是命令逐个“换皮”：

1. 先做 `ui` 组件层
2. 重做 `submit`
3. 重做 `auth`
4. 重做 `history`
5. 重做 `board`
6. 顺手统一 `keys / web / nick / track`

## 对服务端的依赖

`submit` 默认等待完成，需要 CLI 能轮询 submission 最终状态。

如果现有 API 已能满足，就直接复用。

如果现有 API 不足以优雅支持“按 submission id 轮询最终结果”，需要把这件事作为实现计划里的前置服务端任务，而不是在 CLI 层做脆弱猜测。

## 边界

这轮不做：

- 全屏 TUI
- Bubble Tea 导航界面
- WebSocket 实时推送
- 一次性把所有命令彻底重做
- evaluator 原始长文默认整屏输出

## 验证标准

完成后应满足：

- `submit` 默认提交后等待完成并给出最终结果
- `submit` 失败时 detail 自动展开
- `auth` 有明确步骤反馈和完成态
- `history`、`board` 的终端输出信息层级明显提升
- CLI 整体体验更接近 `uv` / `cargo` 一类开发者工具，而不是“脚本输出套个壳”
