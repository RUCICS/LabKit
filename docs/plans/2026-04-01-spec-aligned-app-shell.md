# LabKit 全站 Spec 对齐 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 让 `apps/web` 的顶层壳、学生侧页面和 admin 页面全部严格对齐 `docs/reference/labkit-design-spec.md` 的设计语言。

**Architecture:** 以现有 Vue 路由和 API 为基础，不改业务流程，只重构页面结构、共享壳层和样式系统。优先保证导航、信息层级、卡片节奏、mono/sans 分工、状态 badge 和页面密度统一，再通过现有 Vitest 覆盖关键结构回归。

**Tech Stack:** Vue 3, TypeScript, Vite, Vitest, CSS variables, existing router/views/components

---

### Task 1: 顶层壳与导航居中

**Files:**
- Modify: `apps/web/src/main.ts`
- Modify: `apps/web/src/styles/main.css`
- Test: `apps/web/src/app.smoke.test.ts`

1. 先写或更新测试，约束顶栏存在品牌、居中导航组和状态 badge。
2. 运行相关前端测试，确认旧实现不满足新结构或至少缺少断言。
3. 改 `main.ts` 和共享壳样式，把导航组改成稳定居中布局。
4. 重跑对应测试。

### Task 2: Lab 列表页按 spec 重构

**Files:**
- Modify: `apps/web/src/views/LabListView.vue`
- Test: `apps/web/src/views/LabListView.test.ts`

1. 先补测试，约束列表页不再依赖说明型 lede，并符合 card-first 布局。
2. 运行测试看红或至少覆盖不到位。
3. 按 spec 重构列表页结构和样式。
4. 重跑列表页测试。

### Task 3: Profile/History 页按 spec 收口

**Files:**
- Modify: `apps/web/src/views/ProfileView.vue`
- Test: `apps/web/src/views/ProfileView.test.ts`

1. 先补测试，约束时间线/设备卡片结构和文案层级。
2. 运行测试。
3. 改页面结构与样式，使其更像数据面板而不是账户页。
4. 重跑测试。

### Task 4: Auth Confirm 页改成极简验证页

**Files:**
- Modify: `apps/web/src/views/AuthConfirmView.vue`
- Test: `apps/web/src/app.smoke.test.ts`

1. 先补断言，约束授权成功页的核心信息仍可见。
2. 运行测试。
3. 改成全屏中心化、弱 chrome 的验证页。
4. 重跑测试。

### Task 5: Admin Labs/Queue 与全站风格统一

**Files:**
- Modify: `apps/web/src/views/AdminLabsView.vue`
- Modify: `apps/web/src/views/AdminQueueView.vue`
- Test: `apps/web/src/views/AdminLabsView.test.ts`
- Test: `apps/web/src/views/AdminQueueView.test.ts`

1. 先补测试，覆盖标题层级、关键操作入口和基本数据块仍存在。
2. 运行 admin 测试。
3. 重构 admin 页面布局和视觉节奏，不改功能行为。
4. 重跑 admin 测试。

### Task 6: 全量前端验证与本地预览

**Files:**
- No code changes required unless verification fails

1. 运行 `cd apps/web && npm test`
2. 运行 `cd apps/web && npm run build`
3. 如有需要，运行 `docker compose -f deploy/docker-compose.yml --env-file deploy/.env up -d --build web`
4. 手工检查 `http://localhost:8083/`、`/labs/local-smoke/board`、`/profile`、`/admin`
