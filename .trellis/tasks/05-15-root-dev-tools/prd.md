# 根目录安装通用开发工具

## Goal

将 Vitest、TypeScript、Biome、Testing Library 和 jsdom 安装到 pnpm workspace 根目录，作为多个项目通用的开发与测试工具依赖，避免重复安装到子包。

## Requirements

* 在根目录 `package.json` 的 `devDependencies` 中安装 `vitest`、`typescript`、`@biomejs/biome`、`@testing-library/jest-dom`、`@testing-library/react`、`@testing-library/user-event`、`jsdom`。
* 使用 pnpm workspace 根安装方式，不把这些工具安装到 `apps/*` 或 `services/*` 子包。
* 更新 `pnpm-lock.yaml`，保持锁文件与依赖清单一致。
* 不新增测试或配置文件，本任务仅安装共享开发依赖。

## Acceptance Criteria

* [ ] 根目录 `package.json` 包含 `vitest`、`typescript`、`@biomejs/biome`、`@testing-library/jest-dom`、`@testing-library/react`、`@testing-library/user-event`、`jsdom`。
* [ ] `pnpm-lock.yaml` 已同步更新。
* [ ] 子包 `package.json` 不再声明 `vitest`、`typescript`、`@biomejs/biome`、`@testing-library/jest-dom`、`@testing-library/react`、`@testing-library/user-event`、`jsdom`。
* [ ] 可通过 pnpm 解析安装后的依赖清单。

## Definition of Done

* 依赖安装命令执行完成。
* 检查 git diff，确认变更范围只包含根依赖清单、锁文件和 Trellis 任务记录。
* 若无需业务测试，说明原因。

## Technical Approach

使用 `pnpm add -w -D vitest typescript @biomejs/biome @testing-library/jest-dom @testing-library/react @testing-library/user-event jsdom` 在 workspace 根目录添加共享开发与测试依赖。

## Decision (ADR-lite)

**Context**: Vitest、TypeScript、Biome、Testing Library 和 jsdom 是多个项目共享的开发与测试工具。  
**Decision**: 将这些包安装到 workspace 根目录的 `devDependencies`，不安装到子包。  
**Consequences**: 后续根级脚本、lint/test/typecheck 集成可以统一复用这些工具；子包若需要独立版本，需要另行显式声明。

## Out of Scope

* 不新增 Vitest、TypeScript、Biome 或 Testing Library 配置。
* 不调整现有 lint/test 脚本。
* 不修改子包依赖。

## Technical Notes

* 仓库使用 pnpm workspace，根目录已有 `packageManager` 和根级 `devDependencies`。
* 用户明确要求这些工具是多个项目通用工具，应安装到根目录。
* Context7 CLI 查询 pnpm 文档时返回 `fetch failed`；安装语法采用 pnpm workspace 根安装惯例。
