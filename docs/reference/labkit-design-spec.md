# LabKit Design Specification

Version 0.1 · 2026-04

---

## 1. Design Philosophy

LabKit 是面向 ICS 课程的打榜基础设施，用户是计算机系本科生。设计语言围绕一个核心隐喻：**精密的系统监控面板**——信息密度高、排版严谨、带有终端的冷峻美感，但不模拟终端。

### 1.1 设计原则

- **Precision over decoration.** 每一个视觉元素都服务于信息传递。装饰性元素只在增加层次感或引导注意力时使用。
- **Data is the interface.** 排行榜、分数、状态是页面的主角。UI chrome（导航、边框、按钮）尽可能退后。
- **Respect the craft.** 学生在写调度器、malloc、位运算——他们的劳动成果（分数）值得被精心呈现。
- **Terminal warmth, not terminal cosplay.** 等宽字体、深色背景、网格感——这些元素取自终端美学，但不做终端模拟器。不用绿色文字，不用闪烁光标，不用 `$` prompt。

### 1.2 情感目标

- 打开排行榜时感到："这个东西很专业"
- 看到自己的分数时感到："我的成果被认真对待了"
- 浏览榜单时感到："竞争很激烈，但信息很清晰"

---

## 2. Color System

### 2.1 Base Palette

深色主题为唯一主题。不提供浅色模式——深色是设计语言的一部分，不是用户偏好。

| Token | Hex | 用途 |
|-------|-----|------|
| `--bg-root` | `#06090f` | 页面背景，最深层 |
| `--bg-surface` | `#0b1120` | 卡片、表格容器 |
| `--bg-elevated` | `#111a2e` | 表头、footer、弹层 |
| `--bg-hover` | `#162038` | 行 hover、按钮 hover |
| `--bg-active` | `#1a2844` | 按钮 active、选中态 |

底色使用深蓝黑（非纯黑），通过蓝色底色赋予画面微妙的冷调。纯黑（`#000`）禁止出现。

### 2.2 Text Hierarchy

| Token | Hex | Opacity | 用途 |
|-------|-----|---------|------|
| `--text-primary` | `#e2e8f0` | — | 标题、数据、关键信息 |
| `--text-secondary` | `#8494a7` | — | 正文、次要数据列 |
| `--text-tertiary` | `#4a5568` | — | 标签、时间戳、辅助说明 |
| `--text-inverse` | `#06090f` | — | 深色文字（用于亮色背景上） |

文字颜色不使用 opacity 叠加方案——直接指定色值，避免在不同背景上产生不可预期的效果。

### 2.3 Accent Colors — Track System

CoLab 的三个赛道各有一个 accent color。这组颜色是 LabKit 视觉系统的核心标识。

| Track | Color | Hex | Dim (12% opacity) | 语义 |
|-------|-------|-----|--------------------|------|
| Throughput | 琥珀 | `#f59e0b` | `rgba(245, 158, 11, 0.12)` | 速度、力量 |
| Latency | 青色 | `#06b6d4` | `rgba(6, 182, 212, 0.12)` | 响应、敏捷 |
| Fairness | 紫色 | `#a78bfa` | `rgba(167, 139, 250, 0.12)` | 均衡、秩序 |

Dim 变体用于背景填充（track badge 背景、自己所在行的高亮、赛道标签页的 active 态）。

对于非 CoLab 的 Lab（单指标或双指标），使用 throughput 的琥珀色作为默认 accent。

### 2.4 Semantic Colors

| Token | Hex | 用途 |
|-------|-----|------|
| `--color-open` | `#34d399` | 状态：开放中 |
| `--color-scored` | `#34d399` | verdict：正常出分 |
| `--color-build-failed` | `#f87171` | verdict：编译失败 |
| `--color-rejected` | `#fb923c` | verdict：正确性未通过 |
| `--color-error` | `#f87171` | verdict：系统错误 |
| `--color-queued` | `#8494a7` | 状态：等待中 |

### 2.5 Borders

| Token | Value | 用途 |
|-------|-------|------|
| `--border-subtle` | `rgba(148, 163, 194, 0.06)` | 表格行分隔线 |
| `--border-default` | `rgba(148, 163, 194, 0.10)` | 容器边框 |
| `--border-strong` | `rgba(148, 163, 194, 0.16)` | 表头下分隔线 |

所有边框使用 1px 宽度。不使用 2px 及以上的粗线（唯一例外：自己所在行的左侧高亮线为 2px）。

### 2.6 Rank Colors

| Rank | Token | Hex | 处理 |
|------|-------|-----|------|
| 1 | `--rank-1` | `#fbbf24` | 金色，数字带 `text-shadow` |
| 2 | `--rank-2` | `#94a3b8` | 银灰，纯色 |
| 3 | `--rank-3` | `#d97706` | 铜色，纯色 |
| 4+ | — | `var(--text-tertiary)` | 常规 |

前三名只在排名数字上做颜色区分，不使用图标或 emoji。克制。

---

## 3. Typography

### 3.1 Font Stack

| 用途 | Font Family | Fallback |
|------|------------|----------|
| 等宽 / 数据 / 代码 | JetBrains Mono | Menlo, Consolas, monospace |
| 正文 / UI | IBM Plex Sans | Noto Sans SC, system-ui, sans-serif |

等宽字体承担的角色远超"代码块"——它是排行榜数据列、排名数字、分数、时间戳、状态标签的默认字体。等宽字体保证数字严格对齐。

IBM Plex Sans 用于昵称、正文说明、按钮文字。它比 Inter 更有性格，比 SF Pro 更跨平台。中文 fallback 到 Noto Sans SC（思源黑体），与 IBM Plex Sans 的字重和 x-height 匹配良好。

### 3.2 Type Scale

| Level | Size | Weight | Font | Letter Spacing | 用途 |
|-------|------|--------|------|----------------|------|
| Display | 1.7rem | 700 | Mono | -0.04em | 页面标题（Lab 名称） |
| Headline | 1.5rem | 700 | Mono | -0.03em | 统计数字大字 |
| Data Large | 0.95rem | 600 | Mono | 0 | 排行榜主排序列数据 |
| Data | 0.9rem | 400 | Mono | 0 | 排行榜次要列数据 |
| Body | 0.9rem | 500 | Sans | 0 | 昵称、正文 |
| Label | 0.72rem | 600 | Mono | 0.08-0.1em | 表头、标签、状态 badge |
| Caption | 0.68rem | 400 | Mono | 0.02em | 时间戳、辅助信息、API hint |

所有 label 级别的文字使用 `text-transform: uppercase`。

### 3.3 Numeric Display

排行榜中的数字使用 `font-variant-numeric: tabular-nums`，确保所有数字等宽、小数点对齐。

分数的格式规范：
- 始终显示两位小数：`1.82`，不写 `1.8` 或 `1.820`
- 单位后缀用更小的字号（0.7rem）和更浅的颜色（`--text-tertiary`）：`1.82` `x`
- 单位紧跟数字，间距 1px（`margin-left: 1px`），不加空格

---

## 4. Spatial System

### 4.1 Grid & Container

- 最大内容宽度：1120px，水平居中
- 两侧 padding：24px（移动端 12px）
- 不使用 12 列网格——表格布局用 `table-layout: fixed` + 明确列宽

### 4.2 Spacing Scale

基于 4px 网格：

| Token | Value | 用途 |
|-------|-------|------|
| `--space-1` | 4px | 紧凑间距（badge 内 padding） |
| `--space-2` | 8px | 元素内间距 |
| `--space-3` | 12px | 表格单元格 padding |
| `--space-4` | 16px | 组件间距 |
| `--space-5` | 20px | 区块间距 |
| `--space-6` | 24px | 段落间距 |
| `--space-8` | 32px | 大区块间距 |
| `--space-10` | 40px | 页面顶部留白 |

### 4.3 Border Radius

| Element | Radius |
|---------|--------|
| 容器（表格、卡片） | 10px |
| 按钮、badge | 6px |
| 小元素（色块、dot） | 2-4px |
| Logo icon | 6px |
| 圆形（状态点） | 50% |

不使用 `border-radius: 9999px`（pill shape）用于容器或卡片。Pill shape 只用于极小的标签。

---

## 5. Background & Atmosphere

### 5.1 Grid Texture

页面背景叠加微弱的网格纹理，暗示"坐标系"、"精密"的感觉：

```css
background:
  linear-gradient(rgba(148, 163, 194, 0.02) 1px, transparent 1px),
  linear-gradient(90deg, rgba(148, 163, 194, 0.02) 1px, transparent 1px);
background-size: 48px 48px;
```

网格透明度极低（2%），只在深色背景上隐约可见。不能抢夺前景内容的注意力。

### 5.2 Top Glow

页面顶部有一个柔和的椭圆形辐射光晕，颜色跟随当前赛道的 accent color：

```css
width: 800px;
height: 500px;
background: radial-gradient(ellipse, var(--accent-glow) 0%, transparent 70%);
opacity: 0.3;
```

光晕不跟随滚动（`position: fixed`）。它的作用是给页面一个"色温"，暗示当前赛道的身份。

---

## 6. Component Specification

### 6.1 Top Navigation Bar

```
┌─────────────────────────────────────────────────────────────┐
│  [L] LabKit  /  colab-2026-p2          ● OPEN   历史  文档  │
└─────────────────────────────────────────────────────────────┘
```

- Logo：28×28px 圆角方块，accent color 填充，白色字母 "L"
- Lab ID 以 `font-mono` 显示在 `/` 分隔符后
- 右侧：状态 badge + 导航链接
- 状态 badge 的绿色圆点带呼吸灯动画（2s 周期，opacity 0.4-1）
- 底部有 1px `--border-subtle` 分隔线
- 高度由 padding 决定（20px 上下），不固定高度值

### 6.2 Track Tab Bar

CoLab 这类多赛道 Lab 的核心导航组件。每个赛道对应一个独立的排行榜。

```
┌─────────────────────────────────────────────────────┐
│  [● 吞吐量]    ● 延迟    ● 公平性                    │
└─────────────────────────────────────────────────────┘
```

- 容器：`--bg-surface` 背景，`--border-default` 边框，8px 圆角，3px 内间距
- Tab 按钮：mono 字体，0.78rem，8×20px padding
- Active 态：对应赛道的 dim 色做背景，赛道色 20% opacity 做边框
- 色点（8×8px 圆形）在文字左侧，active 时带 `box-shadow` 光晕
- 单指标 Lab 不显示 tab bar

### 6.3 Leaderboard Table

排行榜是整个系统最重要的组件。

**表头：**
- 背景 `--bg-elevated`
- 字体 mono，0.68rem，weight 600，uppercase，`letter-spacing: 0.1em`
- 颜色 `--text-tertiary`
- 当前排序列高亮为赛道色，末尾追加 ` ↓`（desc）或 ` ↑`（asc）
- Sticky 定位

**数据行：**
- 12px 垂直 padding，16px 水平 padding
- 行分隔用 1px `--border-subtle`
- Hover 态 `--bg-hover`
- 自己所在行：赛道 dim 色背景 + 左侧 2px 赛道色边框
- 进入动画：`fadeInUp`（8px 位移 + opacity），每行递增 30ms delay

**列规格：**

| 列 | 宽度 | 对齐 | 字体 | 说明 |
|----|------|------|------|------|
| Rank | 64px | 居中 | Mono 700 | 前三用 rank color |
| Nickname | auto (弹性) | 左对齐 | Sans 500 | 含 track indicator |
| Metric × N | 140px each | 右对齐 | Mono | 主列 600/0.95rem，其余 400/0.9rem |
| Updated | 130px | 右对齐 | Mono 400 caption | 移动端隐藏 |

**Track Indicator：** nickname 左侧 4×20px 竖色条，颜色对应该学生声明的赛道。这个色条是排行榜中最重要的视觉元素之一——它让你一眼扫过去就能看到"谁在冲吞吐量、谁在走公平性"。

**Footer：**
- 背景 `--bg-elevated`，顶部 1px `--border-default`
- 左侧：统计信息（参与人数、最后更新时间、截止日期）
- 右侧：API 端点 hint（`GET /api/labs/.../board`），极低对比度，hover 时稍亮

### 6.4 Status Badge

```
● OPEN
```

- Mono 字体，0.72rem，weight 600，uppercase，`letter-spacing: 0.08em`
- 5×10px padding，4px 圆角
- 文字色和背景色由语义色决定（open = 绿，closed = 灰）
- 绿色圆点带呼吸灯动画

### 6.5 Verdict Badge

用于提交历史页面，标记每次提交的 verdict。

| Verdict | 文字色 | 背景色 | 边框色 |
|---------|--------|--------|--------|
| `scored` | `--color-scored` | scored dim | scored 15% |
| `build_failed` | `--color-build-failed` | 红 dim | 红 15% |
| `rejected` | `--color-rejected` | 橙 dim | 橙 15% |
| `error` | `--color-error` | 红 dim | 红 15% |
| `queued` | `--text-secondary` | `--bg-elevated` | `--border-default` |

样式与 status badge 一致（mono, uppercase, small）。

### 6.6 Stat Card

页面标题右侧的统计数据块：

```
 47        186       34d
PARTICIPANTS  SUBMISSIONS  REMAINING
```

- 数字：mono 700，1.5rem，`--text-primary`
- 标签：mono 600，0.68rem，uppercase，`letter-spacing: 0.08em`，`--text-tertiary`
- 单位后缀（如 `d`）用 0.7em + `--text-tertiary` + weight 400

### 6.7 Lab Card（列表页）

```
┌──────────────────────────────────────┐
│  CoLab 调度器竞赛            ● OPEN  │
│  colab-2026-p2                       │
│                                      │
│  scheduler.cpp · 3 metrics           │
│  47 participants · closes 06-01      │
│  ───────────────────────────────     │
│  ● throughput  ● latency  ● fairness │
└──────────────────────────────────────┘
```

- 背景 `--bg-surface`，边框 `--border-default`，10px 圆角
- Hover 态：边框变为 `--border-strong`，微弱上移（`transform: translateY(-1px)`）
- 底部展示该 Lab 的 metric 列表，每个 metric 前用对应的赛道色圆点
- 点击进入该 Lab 的排行榜页面

---

## 7. Page Layouts

### 7.1 排行榜页（核心页面）

这是 LabKit 最重要的页面。结构从上到下：

```
┌─ Top Nav ────────────────────────────────────────┐
│  [L] LabKit / colab-2026-p2        ● OPEN  ...   │
├──────────────────────────────────────────────────┤
│                                                  │
│  CoLab 调度器竞赛                  47  186   34d │
│  ICS2 · 2026 Spring                              │
│                                                  │
│  ┌─ Tab: 吞吐量 ─┐┌─ Tab: 延迟 ─┐┌─ Tab: 公平性 ┐│
│  │    (active)    ││             ││              ││
│  └────────────────┘└─────────────┘└──────────────┘│
│                                                  │
│  ┌─ Board ──────────────────────────────────────┐│
│  │ #  Nickname     Throughput  Latency  Fairness││
│  │─────────────────────────────────────────────-││
│  │ 1  🐱 调度猫     2.31x      1.12x    0.98x  ││
│  │ 2  sched_master  2.15x      1.45x    1.21x  ││
│  │ ...                                          ││
│  ├──────────────────────────────────────────────┤│
│  │ 47 on board · Last update 04-30 · ...        ││
│  └──────────────────────────────────────────────┘│
└──────────────────────────────────────────────────┘
```

**关键交互：三个赛道 = 三张独立的排行榜。**

- 点击赛道 tab 切换到该赛道的排行榜
- 切换时整个表格重新按该赛道的 metric 排序
- 当前排序列高亮加粗，其他 metric 列仍然展示但颜色较浅
- 页面的 accent 色（光晕、active 态）跟随切换
- 排名完全独立——吞吐量榜的第 1 名和延迟榜的第 1 名可以是不同的人
- 单指标 Lab 不显示 tab bar，直接展示唯一的排行榜

### 7.2 Lab 列表页

```
┌─ Top Nav ────────────────────────────────────────┐
│  [L] LabKit                                       │
├──────────────────────────────────────────────────┤
│                                                  │
│  Labs                                            │
│                                                  │
│  ┌─ Card ─────────┐ ┌─ Card ─────────┐          │
│  │ CoLab 调度器竞赛 │ │ Malloc 性能挑战  │          │
│  │ ● OPEN         │ │ ● UPCOMING     │          │
│  └────────────────┘ └────────────────┘          │
│  ┌─ Card ─────────┐                              │
│  │ Data Lab       │                              │
│  │ ● CLOSED       │                              │
│  └────────────────┘                              │
└──────────────────────────────────────────────────┘
```

- 卡片使用 CSS Grid 布局，`repeat(auto-fill, minmax(340px, 1fr))`
- 间距 16px

### 7.3 提交详情页

登录后可见。展示该学生在某 Lab 的所有提交历史。

```
┌─ Top Nav ──────────────────────────────────────┐
├────────────────────────────────────────────────┤
│                                                │
│  我的提交 · CoLab 调度器竞赛                     │
│  声明赛道: ● throughput   今日剩余: 2/3         │
│                                                │
│  ┌─ Submission #186 ──── scored ───── 04-30 ─┐ │
│  │  throughput 1.82x  latency 1.21x  ...     │ │
│  │  [展开详情]                                │ │
│  ├─ Submission #142 ──── scored ───── 04-28 ─┤ │
│  │  throughput 1.65x  latency 1.18x  ...     │ │
│  ├─ Submission #98 ── build_failed ── 04-27 ─┤ │
│  │  编译失败，请检查语法错误                    │ │
│  └───────────────────────────────────────────┘ │
└────────────────────────────────────────────────┘
```

- 时间线布局，最新提交在上
- 每条提交显示：序号、verdict badge、时间戳、所有 metric 分数
- `scored` 的提交可展开查看 `detail`（Markdown 渲染区）
- 非 `scored` 的提交显示 `message`（诊断信息）

### 7.4 Device Flow 验证页

学生首次授权时在浏览器中打开的页面。极简。

```
┌──────────────────────────────────────┐
│                                      │
│             [L] LabKit               │
│                                      │
│      请输入终端中显示的验证码          │
│                                      │
│      ┌──┐ ┌──┐ ┌──┐ ┌──┐            │
│      │ A│ │ B│ │ C│ │ D│            │
│      └──┘ └──┘ └──┘ └──┘            │
│              —                       │
│      ┌──┐ ┌──┐ ┌──┐ ┌──┐            │
│      │ 1│ │ 2│ │ 3│ │ 4│            │
│      └──┘ └──┘ └──┘ └──┘            │
│                                      │
│          [ 确认授权 ]                 │
│                                      │
└──────────────────────────────────────┘
```

- 全屏居中，无导航栏
- 验证码输入框用大号 mono 字体，每个字符一个格子
- 背景使用网格纹理动画（微弱移动），暗示"系统正在等待连接"
- 授权成功后显示一行确认文字 + 自动关闭倒计时

---

## 8. Motion & Animation

### 8.1 原则

- **Functional, not decorative.** 动效服务于状态反馈和信息层级，不是视觉炫技。
- **Fast.** 大多数过渡在 150-200ms 完成。排行榜行入场动画 300ms。
- **Easing:** `ease` 用于通用过渡，`ease-out` 用于入场。不使用 `linear`。

### 8.2 动效清单

| 元素 | 动效 | 时长 | 触发 |
|------|------|------|------|
| 排行榜行 | fadeInUp（8px + opacity） | 300ms | 页面加载 / 切换赛道 |
| 排行榜行 delay | 每行递增 30ms | — | 逐行入场 |
| 状态圆点 | opacity 呼吸（1 → 0.4 → 1） | 2000ms | 持续循环 |
| 按钮 hover | background + color | 150ms | hover |
| 赛道切换 | accent 色过渡 | 400ms | 点击 tab |
| 页面光晕 | background 色过渡 | 600ms | 赛道切换 |
| Lab 卡片 hover | translateY(-1px) + border | 150ms | hover |

### 8.3 禁止的动效

- 页面转场动画
- 弹窗飞入/飞出
- 数字计数器滚动（countup）——直接显示最终值
- 任何超过 600ms 的单次动效
- Parallax 效果
- 骨架屏加载动画（直接显示加载状态文字）

---

## 9. Responsive Behavior

### 9.1 断点

| Breakpoint | 宽度 | 调整 |
|------------|------|------|
| Desktop | ≥ 768px | 完整布局 |
| Mobile | < 768px | 紧凑布局 |

只用两个断点。不做 tablet 特殊适配。

### 9.2 Mobile Adaptations

- 容器 padding 从 24px 减为 12px
- `Updated` 列隐藏
- Metric 列宽从 140px 减为 100px
- 页面标题和统计数字垂直堆叠（移动端标题在上，数字在下）
- Track tab 按钮 padding 缩小
- 排行榜容器可以水平滚动（如果 metric 过多）

---

## 10. Accessibility

- 所有交互元素可通过键盘 Tab 访问
- Track tab 使用 `role="tablist"` + `role="tab"` 语义
- 排行榜使用 `<table>` 语义（非 div 模拟）
- 颜色对比度：`--text-primary` 在 `--bg-surface` 上达到 WCAG AA（≥ 4.5:1）
- Rank color 在 `--bg-surface` 上也达到 AA
- 不依赖纯颜色传递信息——track indicator 的颜色辅以位置区分

---

## 11. Copywriting & i18n

### 11.1 语言

- UI chrome（导航、表头、标签）使用英文：Rank, Nickname, Updated, Participants, Submissions
- 内容和说明使用中文：Lab 名称、状态提示语、验证码说明
- 这个中英混排是有意为之——模仿 `htop`、`docker ps` 等工具的信息呈现方式

### 11.2 排版规范

遵循项目中文排版指南：
- 中英文之间增加空格：`在 LeanCloud 上`
- 中文与数字之间增加空格：`花了 5000 元`
- 全角标点与其他字符之间不加空格
- 使用弯引号，不使用直角引号
- 专有名词使用正确大小写：GitHub, Docker, CoLab

---

## 12. Implementation Notes

### 12.1 CSS Variable Usage

所有颜色通过 CSS Variables 引用，便于：
- 赛道切换时通过 JS 修改 `--active-color` 等变量实现全局色调切换
- 未来支持深/浅主题（虽然当前不提供浅色模式）
- Lab-specific 的自定义配色

### 12.2 Performance

- 排行榜数据通过 JSON API 一次性加载，客户端排序
- 不做翻页，直接渲染所有行（课程规模通常 < 500 人）
- 表格行入场动画使用 CSS animation，不用 JS 动画库
- 字体文件通过 CDN 加载，`preconnect` 优化
