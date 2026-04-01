# 本地认证与 Lab 编写文档拆分 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为 LabKit 增加两份中文参考文档和一套可运行示例，分别解释本地认证/Admin 鉴权，以及 manifest/evaluator 协议。

**Architecture:** 不改核心 API 和 worker 行为，只新增两份参考文档、示例 manifest、示例 evaluator，并在 README 中增加入口。文档直接基于当前代码真实行为编写，不做“未来计划式”描述。

**Tech Stack:** Markdown, TOML, Dockerfile, Python 3, existing Go/worker runtime contracts

---

### Task 1: 写本地认证与 Admin 鉴权文档

**Files:**
- Create: `docs/reference/local-auth.md`

1. 写清当前学生认证链路。
2. 写清当前浏览器 session 的来源。
3. 写清 admin token 鉴权方式。
4. 写清本地 dev auth workaround 的具体步骤。

### Task 2: 写 Lab manifest 与 evaluator 协议文档

**Files:**
- Create: `docs/reference/lab-authoring.md`

1. 列出测试 Lab manifest 的核心字段。
2. 说明 worker 如何运行 evaluator 镜像。
3. 说明 `/submission` 挂载约定、stdout 最后一行 JSON 约定、verdict/scores/detail 约定。
4. 给出最小 JSON 输出样例。

### Task 3: 新增可运行示例

**Files:**
- Create: `examples/labs/local-smoke.lab.toml`
- Create: `examples/evaluator/local-smoke/Dockerfile`
- Create: `examples/evaluator/local-smoke/evaluator.py`

1. 提供一个最小 manifest。
2. 提供一个最小 evaluator 镜像示例。
3. 确保示例与当前协议一致。

### Task 4: 更新 README 入口

**Files:**
- Modify: `README.md`

1. 在 README 中增加新文档和示例入口。
2. 把“本地认证 workaround”指向参考文档。

### Task 5: 验证

**Files:**
- No code changes required unless verification fails

1. 检查新文档和示例文件都存在。
2. 重新阅读文档中的命令与路径，确保和仓库结构一致。
3. 运行 `bash scripts/deploy_smoke_test.sh`，确认 README/部署结构改动未破坏现有 smoke test。
