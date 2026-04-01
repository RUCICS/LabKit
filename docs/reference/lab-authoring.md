# Lab 编写与 Evaluator 协议说明

这份文档说明当前仓库里一个 lab 要怎么接进 LabKit，包括：

- manifest 要写什么
- evaluator 镜像怎么配置
- worker 运行 evaluator 时的约定是什么
- stdout 最后一行 JSON 应该长什么样

## 1. 最小可运行示例

仓库里提供了一套最小示例：

- manifest: [local-smoke.lab.toml](/home/starrydream/ICS2/LabKit/examples/labs/local-smoke.lab.toml)
- evaluator Dockerfile: [Dockerfile](/home/starrydream/ICS2/LabKit/examples/evaluator/local-smoke/Dockerfile)
- evaluator 脚本: [evaluator.py](/home/starrydream/ICS2/LabKit/examples/evaluator/local-smoke/evaluator.py)

这套示例约定学生提交一个 `submission.json`，内容里带一个 `score` 数字。evaluator 读取它，然后输出一条合法的 `scored` JSON 结果。

## 2. Manifest 里最重要的字段

一个最小 manifest 至少要把下面这些段写清楚：

### 2.1 `[lab]`

- `id`: lab 的稳定 ID
- `name`: 展示名称

### 2.2 `[submit]`

- `files`: 学生允许上传的文件名列表
- `max_size`: 提交包总大小限制

Platform 会严格按 `files` 校验。worker 在评测前也只会把这些文件复制进评测容器。

### 2.3 `[eval]`

- `image`: evaluator 镜像名
- `timeout`: 评测超时秒数

这里的 `image` 必须是 worker 所在宿主机 Docker 能直接找到的镜像名，例如：

```toml
[eval]
image = "labkit-local-smoke:latest"
timeout = 30
```

### 2.4 `[[metric]]`

每个 metric 都要声明：

- `id`
- `name`
- `sort`

其中 `id` 必须和 evaluator 输出 JSON 里的 `scores` key 严格一致。

### 2.5 `[board]`

- `rank_by`: 排行榜默认排序 metric
- `pick`: 当前是否允许学生声明赛道

单指标 lab 一般直接设 `pick = false`。

### 2.6 `[schedule]`

至少要有：

- `visible`
- `open`
- `close`

本地测试时可以直接给一个很宽的时间窗。

## 3. Evaluator 镜像应该怎么写

当前 worker 会直接调用宿主机 Docker Engine 运行 evaluator。最小镜像只需要做到一件事：

- 容器启动后自己完成评测，并把最终 JSON 打到 stdout 最后一行

示例镜像：

```dockerfile
FROM python:3.12-alpine
WORKDIR /evaluator
COPY evaluator.py /evaluator/evaluator.py
CMD ["python3", "/evaluator/evaluator.py"]
```

本地构建示例镜像：

```bash
docker build -t labkit-local-smoke:latest examples/evaluator/local-smoke
```

只要 manifest 的 `[eval].image` 和这里构建出来的 tag 一致，worker 就能拿它来跑。

## 4. Worker 实际怎样运行 evaluator

当前 worker 的真实行为是：

1. `docker create`
2. 带上资源限制：
   - `--network=none`
   - `--memory <limit>`
   - `--cpus <limit>`
3. 把 staging 后的提交目录 `docker cp` 到容器根目录
4. `docker start -a`
5. 读取容器 stdout
6. 只解析 stdout 的最后一行 JSON
7. 删除容器

这里有两个重要约定：

- evaluator 容器默认没有网络
- 提交文件会被复制成容器内的 `/submission/...`

所以 evaluator 应该从 `/submission/` 读取学生提交，而不是依赖别的路径。

## 5. `/submission` 目录约定

如果 manifest 写的是：

```toml
[submit]
files = ["submission.json"]
```

那么 evaluator 在容器里应该读取：

```text
/submission/submission.json
```

文件名必须和 manifest 完全一致。

## 6. stdout 最后一行 JSON 协议

当前 worker 只看 stdout 最后一行非空内容，并把它当 JSON 解析。

这意味着：

- 你前面可以打印编译日志、调试日志
- 但最后一行必须是合法 JSON
- 如果最后一行不是合法 JSON，worker 会把这次评测视为 `error`

最小 `scored` 结果示例：

```json
{"verdict":"scored","scores":{"score":95.0},"detail":{"format":"markdown","content":"# OK\n\nSmoke evaluator passed."}}
```

## 7. `verdict` / `scores` / `detail` 约定

### 7.1 verdict 词表

当前只接受这四个值：

- `build_failed`
- `rejected`
- `scored`
- `error`

### 7.2 scores 规则

只有 `verdict = "scored"` 时才允许出现 `scores`。

而且 `scores` 必须：

- 不为空
- key 和 manifest 的 `[[metric]].id` 完全一致
- 不能缺 key
- 不能多 key

例如 manifest 只有一个 metric：

```toml
[[metric]]
id = "score"
name = "Score"
sort = "desc"
```

那么 evaluator 只能输出：

```json
{"verdict":"scored","scores":{"score":95.0}}
```

### 7.3 detail 规则

`detail` 是可选字段，当前支持两种 `format`：

- `text`
- `markdown`

例如：

```json
{
  "verdict": "scored",
  "scores": { "score": 95.0 },
  "detail": {
    "format": "markdown",
    "content": "## Breakdown\n\n- parser: ok\n- runtime: ok"
  }
}
```

### 7.4 message 的作用

`message` 是可选字段，通常用于非 `scored` 结果的简短诊断：

```json
{"verdict":"build_failed","message":"gcc failed"}
```

## 8. 一个最小 evaluator 脚本长什么样

示例脚本见 [evaluator.py](/home/starrydream/ICS2/LabKit/examples/evaluator/local-smoke/evaluator.py)。

它做的事情很简单：

1. 读取 `/submission/submission.json`
2. 从里面拿 `score`
3. 打一行普通日志
4. 最后一行打印合法 JSON

这正好符合当前 worker 的协议。

## 9. 本地 smoke lab 怎么用

### 9.1 构建 evaluator 镜像

```bash
docker build -t labkit-local-smoke:latest examples/evaluator/local-smoke
```

### 9.2 启动本地栈

```bash
bash scripts/dev-up.sh
```

### 9.3 在 admin 里注册 manifest

打开：

```text
http://localhost:8083/admin?token=dev-admin-token
```

然后把 [local-smoke.lab.toml](/home/starrydream/ICS2/LabKit/examples/labs/local-smoke.lab.toml) 的内容贴进去注册。

### 9.4 准备一个本地提交文件

例如：

```json
{
  "score": 95,
  "note": "local smoke test"
}
```

保存为 `submission.json`。

### 9.5 完成一次本地 auth

按 [local-auth.md](/home/starrydream/ICS2/LabKit/docs/reference/local-auth.md) 里的 dev bind 步骤先把 CLI 绑定好。

### 9.6 提交

```bash
go run ./apps/cli/cmd/labkit --server-url http://localhost:8083 --lab local-smoke submit submission.json
```

然后你可以：

- 用 `history` 看提交记录
- 回 admin queue 看 job
- 回 leaderboard 页面看分数

## 10. 这个示例刻意没有覆盖的东西

这套示例是最小 smoke path，不覆盖这些复杂问题：

- 多指标排序
- 编译步骤
- 复杂 benchmark
- 大型依赖环境
- 评测缓存
- 外部数据集

它的目的只是让你把“manifest -> register -> submit -> worker -> leaderboard”这条链路本地跑通。
