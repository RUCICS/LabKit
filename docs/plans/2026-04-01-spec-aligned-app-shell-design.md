# LabKit 全站 Spec 对齐设计

日期：2026-04-01

## 目标

把当前 `apps/web` 从“局部页面有 spec 风格”收口成“整站视觉语言和信息架构都遵循 [labkit-design-spec.md](/home/starrydream/ICS2/LabKit/docs/reference/labkit-design-spec.md)`”的状态。

这轮不再做“接近 spec”的微调，而是直接把顶层壳、列表页、榜单页、历史页、授权页、admin 页全部拉回同一套监控面板语言。

## 关键问题

当前页面之所以还像 demo，不是因为颜色或字体错一点，而是因为这些结构性偏差：

- 顶栏导航没有固定在视觉中轴线上，chrome 太散。
- 多个页面仍然依赖 hero 说明文案建立语义，而不是让数据本身成为主角。
- 列表页、历史页、授权页、admin 页没有统一到 spec 的 mono/sans 分工、间距、状态 badge、卡片节奏。
- 学生侧和 admin 侧像两套独立产品，没有共同的壳层与层级。

## 设计决定

### 1. 顶层壳统一

顶栏统一成三段结构：

- 左侧：`[L] LabKit / <route-meta>`
- 中间：固定居中的导航组
- 右侧：状态 badge

其中中间导航采用绝对居中或等价布局，确保窗口宽度变化时仍然稳定对齐到页面中轴，而不是被左侧品牌宽度推歪。

### 2. 学生侧页面统一成“数据优先”

- `Lab 列表页`：按 spec 的 Lab Card 结构重做，去掉解释性 lede，直接展示 lab 名称、状态、文件、metric、截止信息。
- `Leaderboard`：继续保持上一轮的 board-first 结构。
- `Profile/History`：改成提交历史/设备信息时间线与 verdict badge 风格，不再像普通账户页。
- `Auth Confirm`：改成极简验证页，中心布局，弱化普通 panel 感。

### 3. Admin 视觉统一但功能不减

admin 仍保留：

- lab 注册 / 更新
- queue 查看
- grades 导出
- reeval

但视觉语言改成同一套深色监控面板系统：

- 统一标题和 mono 标签
- 更紧凑的 panel/section head
- 更像控制台而不是单独后台模板

### 4. 文案和排版遵循 spec

- UI chrome 用英文：`Labs`, `Admin`, `History`, `Rank`, `Updated`
- 内容说明用中文
- 去掉“这是榜单/这是演示”式说明文案
- 统一使用 `IBM Plex Sans + JetBrains Mono`

## 影响范围

主要改动文件：

- `apps/web/src/main.ts`
- `apps/web/src/styles/main.css`
- `apps/web/src/views/LabListView.vue`
- `apps/web/src/views/LeaderboardView.vue`
- `apps/web/src/views/ProfileView.vue`
- `apps/web/src/views/AuthConfirmView.vue`
- `apps/web/src/views/AdminLabsView.vue`
- `apps/web/src/views/AdminQueueView.vue`
- 相关测试文件

## 验证标准

完成后要满足：

- 顶部导航组稳定居中
- 学生侧和 admin 侧页面肉眼上属于同一产品
- 列表页、榜单页、历史页、授权页都不再依赖说明性 hero 文案
- `npm test` 和 `npm run build` 继续通过
