# 软件图标方案

## Goal

为 taskdaemon 选择一套可落地的软件图标制作方案，覆盖 Web favicon、Wails Desktop 发布图标和后续品牌延展。当前重点是先确定图标来源与风格方向，再决定是否进入实际制图和资源接入。

## What I already know

* 用户想了解：软件图标可以直接用字体图标做，还是用提示词通过 GPT Image 生成。
* 项目是 Wails v3 + Vite React；Web/Desktop 共用 UI。
* 当前前端没有 favicon/app icon 资源，`apps/web/index.html` 也没有 `<link rel="icon">`。
* 当前 Wails 配置没有显式 app icon；`services/taskdaemon-go/wails.json` 只配置了 frontend、name、author。
* 前端品牌目前在 `apps/web/src/components/layout/AppShell.tsx` 内使用 lucide `Clock3` 加文字 `taskdaemon`。
* Wails v3 文档显示发布构建可用 `wails3 build -icon build/appicon.png` 指定 PNG 图标，Windows 构建会自动转换为 `.ico`；也可在 `build/build.json` 中配置 `"icon": "build/appicon.png"`。

## Requirements (evolving)

* 图标需要适合一个跨平台、低占用、定时任务守护进程工具。
* 图标需要在小尺寸下可识别，例如 16/32/64 px。
* 图标需要能同时生成 Web favicon 和 Desktop 发布图标。
* 第一阶段可以先做单张高分辨率 PNG/SVG 母版，再派生 `.ico`、favicon 和 Wails build icon。
* 用户已选择 AI 生成品牌图标母版作为首选路线。
* 用户已选择视觉方向 1：深色专业 devtool 风格，主色建议为 deep graphite / electric cyan / warm amber。
* 用户已选择现在进入图标图稿生成阶段。
* 用户决定先自行使用提示词在 GPT Image 中出图，后续再把候选图交给项目接入。
* 用户已选定一张包含时钟、终端和循环元素的图标作为当前主图标。

## Visual Direction

### Keywords

* 深色专业开发者工具
* 定时调度 / 自动化循环 / 后台守护进程
* 终端命令运行感，但不直接使用文字或字母
* 主图标只表达调度执行器，不承载 AI 表意；未来 AI agent 通过产品内 badge / 模块图标表达
* 高对比、扁平矢量感、边缘清晰
* 小尺寸优先，16px 下仍保留可识别轮廓

### Forbidden Elements

* 不使用文字、品牌字母或缩写。
* 不使用 mascot、人物、动物或复杂场景。
* 不使用机器人头、大脑、魔法棒等过于直白或容易过时的 AI 符号。
* 不使用分享节点、三点发散节点、神经网络节点、分支连线或 spark 装饰；这些元素在 GPT Image 中容易变得花哨且偏离调度工具定位。
* 不添加无法解释的额外点状节点；若需要强调运行状态，使用短横线、播放三角或终端光标形状，而不是圆点。
* 不使用 3D mockup、照片风格、背景环境图。
* 不使用过密刻度、过多节点、细碎阴影等小尺寸会糊掉的细节。
* 不做偏营销插画或启动页风格，保持桌面应用图标感。

### Future AI Agent Direction

* 主图标仅以“调度执行”为核心，不尝试在 app icon 中表达 AI。
* 推荐符号组合收敛为：clock/timing ring + automation loop + terminal/run marker。
* 未来 AI agent 能力通过产品内功能入口、AI badge、启动页文案或模块图标表达；桌面 app 主图标保持更稳定的 scheduler/daemon 品牌。
* 图标可通过更“主动执行”的 run indicator 表达任务正在运行，但不要要求模型绘制 agent spark、decision node、branching graph 或任何额外点状节点。

## Feasible Approaches

### Approach A: 字体/图标库图标占位

* How it works: 直接选 lucide 的 clock、activity、calendar-clock、terminal 等图标，配一个简单底色导出为 SVG/PNG。
* Pros: 很快、风格和当前 UI 一致、适合开发期 favicon/占位。
* Cons: 辨识度弱，容易像通用闹钟/任务应用；最终发布图标显得不够品牌化。

### Approach B: AI 生成品牌图标母版（Recommended）

* How it works: 用 GPT Image 生成 1024x1024 图标母版，方向是“daemon/cron/automation”抽象符号，避免文字；再人工挑选和微调，导出 PNG/SVG/ICO。
* Pros: 更容易形成独立识别度；适合 Desktop 应用图标和官网/文档品牌延展。
* Cons: 需要多轮筛选；AI 生成结果可能有小尺寸不清晰、细节过多或风格不统一的问题。

### Approach C: 手工矢量图标

* How it works: 在 Figma/Inkscape/代码 SVG 中设计一个简化符号，比如时钟刻度 + 终端提示符 + 任务节点。
* Pros: 最干净、可控、适合多尺寸输出。
* Cons: 需要设计时间；如果没有明确视觉方向，容易做得普通。

## Recommended Direction

建议采用 Approach B + C 的混合方式：

1. 先用 GPT Image 生成 6–12 个方向稿，找出最有识别度的轮廓。
2. 选定一个后，简化成矢量/扁平母版，控制细节，保证 16px 仍能看出来。
3. 输出 `services/taskdaemon-go/build/appicon.png` 作为 Wails build icon，输出 `apps/web/public/favicon.svg/png` 作为 Web favicon。

## Generation Plan

* 先生成 3 个 1024x1024 图标母版方向稿，分别对应：
  1. 工具型桌面应用图标：时钟 + 自动化循环 + 命令运行。
  2. 后台守护进程图标：抽象 clock ring + terminal prompt 形状。
  3. 运维调度图标：时钟刻度 + run pulse/check + 任务节点。
* 产物先作为候选图稿保存，不直接覆盖最终 app icon。
* 选中候选后，再派生最终接入资源：Wails app icon 与 Web favicon。
* 当前 Codex 会话未暴露内置 `image_gen` 工具，且本机未检测到 `OPENAI_API_KEY`；实际生成需切换到可用内置出图工具的环境，或用户配置 API key 后改用 imagegen CLI fallback。
* 当前执行方式调整为：用户自行出图，Codex 提供可复制提示词和后续接入方案。

## Selected Asset

* 源图留档：`.trellis/tasks/05-16-software-icon-design/assets/icon-source.png`。
* Wails Desktop 图标：`services/taskdaemon-go/build/appicon.png`。
* Windows 图标派生产物：`services/taskdaemon-go/build/icon.ico`。
* macOS 图标派生产物：`services/taskdaemon-go/build/icons.icns`。
* Web favicon：`apps/web/public/favicon.png`。
* Web 入口：`apps/web/index.html` 通过 `<link rel="icon" type="image/png" href="/favicon.png" />` 引用。
* 当前 Wails v3 本地版本的 `build` 命令未暴露 `-icon` 参数；发布图标应先通过 `go tool wails3 generate icons -input build/appicon.png` 派生 `.ico/.icns`，Windows 可继续通过 `go tool wails3 generate syso -icon <icon.ico>` 生成资源文件。

## Decision (ADR-lite)

### 图标来源采用 AI 生成母版

**Context**: taskdaemon 当前没有 favicon/app icon 资源。字体图标可以快速占位，但辨识度弱，最终发布图标需要更独立的轮廓与品牌感。

**Decision**: 先使用 GPT Image 生成高分辨率图标母版，围绕“定时调度、后台守护进程、脚本执行/终端”三个意象出多版方向稿，再挑选一个简化为最终可多尺寸使用的图标。

**Consequences**: 后续需要控制 AI 结果的细节密度，避免文字、字母、复杂场景和小尺寸不可读问题；最终接入前仍需要导出标准图标资源。

## Prompt Drafts

### Prompt 1: 工具型桌面应用图标

```text
Create a clean modern desktop app icon for "taskdaemon", a lightweight cross-platform task scheduler daemon. The icon should combine the ideas of a clock, automation loop, and command runner, using a simple memorable symbol. Professional developer tool style, high contrast, readable at 16px, no text, no letters, no mascot, no 3D mockup, no background scene. Centered icon on a rounded square background. Color palette: deep graphite, electric cyan, and warm amber accent. Flat vector-like rendering, crisp edges, minimal details, 1024x1024.
```

### Prompt 2: 更偏守护进程/后台服务

```text
Design a minimal app icon for a background daemon that schedules and runs scripts. Use an abstract circular clock ring with a small terminal prompt shape integrated into it, suggesting reliable automation. No text, no letters, no people, no animals, no complex scene. Must work as a macOS/Windows/Linux desktop icon and favicon. Calm professional palette, dark charcoal base, cyan timing marks, small amber run indicator. Flat modern vector icon, centered, strong silhouette, 1024x1024.
```

### Prompt 3: 更偏备份/运维场景

```text
Create a polished icon for a developer operations scheduling tool used for backups and scripts. The symbol should suggest scheduled execution: a clock dial, a check mark/run pulse, and a compact server/task node motif. No text, no brand letters, no photorealism, no busy detail. High readability at small sizes, simple geometric shapes, rounded square app icon, professional SaaS/devtool aesthetic, graphite background with teal and amber accents, 1024x1024.
```

### Prompt 4: 预留 AI agent 能力

```text
Create a clean modern desktop app icon for "taskdaemon", a cross-platform scheduler daemon that may evolve into an AI agent for running and coordinating tasks. The core symbol should still communicate scheduled execution: a clock/timing ring, an automation loop, and a command-run indicator. Add a subtle AI-agent hint using a small decision node, agent spark, or intelligent run pulse, but do not make it look like a robot, brain, chatbot, or magic wand. Professional developer tool style, dark graphite rounded square background, electric cyan timing/automation lines, small warm amber accent, flat vector-like rendering, crisp edges, strong silhouette, readable at 16px. No text, no letters, no mascot, no people, no animals, no complex scene, no 3D mockup, no photorealism, 1024x1024.
```

## Acceptance Criteria (evolving)

* [x] 确定图标制作路线：字体占位 / AI 生成母版 / 手工矢量。
* [x] 确定视觉关键词和禁用元素。
* [x] 如果进入实现，生成或制作至少 1 个高分辨率图标母版。
* [x] 如果进入接入，配置 Web favicon 和 Wails build icon。

## Definition of Done

* 图标方案已确认。
* 如产生资源，文件路径和构建使用方式记录清楚。
* 如接入代码，lint / typecheck / build 检查通过。

## Out of Scope (explicit)

* 本 brainstorm 阶段不直接生成最终图标文件。
* 不做完整品牌手册。
* 不做安装包图标、托盘图标、通知图标的完整矩阵，除非后续确认进入实现。

## Technical Notes

* Inspected `apps/web/index.html`: 当前未配置 favicon。
* Inspected `services/taskdaemon-go/wails.json`: 当前未配置 app icon。
* Inspected `services/taskdaemon-go/build/config.yml`: Wails dev 配置未涉及 icon。
* Context7 Wails v3 docs: `wails3 build -icon build/appicon.png` 支持 PNG 图标，并可自动转换 Windows `.ico`；`build/build.json` 也支持 `"icon": "build/appicon.png"`。
