# TypeScript Runner 调研：tsx

## 背景

用户指出 TypeScript 任务执行不应默认考虑 `ts-node`，更适合使用 `tsx`。项目需要支持定时执行 TypeScript 脚本，并在第一版做到内置 runner 抽象，插件系统作为后续优化。

## 资料来源

* Context7: `/privatenumber/tsx`

## 关键结论

* `tsx` 可直接执行 TypeScript 文件，命令形式类似 `tsx ./script.ts` 或 `npx tsx ./script.ts`。
* `tsx` 支持标准 Node.js CLI flags，并可向脚本透传参数。
* `tsx` 支持 `--env-file`、`--tsconfig` 等常见执行配置。
* 与 `ts-node` 相比，`tsx` 更偏零配置，适合脚本任务 runner 的默认推荐。

## 映射到本项目

第一版 TypeScript runner 建议：

* runner type: `typescript`
* 默认 executable: `tsx`
* 支持配置：
  * script path
  * args
  * env/env_file
  * working_dir
  * tsconfig
  * timeout
* 若本机未安装 `tsx`，任务执行应给出清晰错误，而不是自动安装依赖。

## 后续优化

* 可支持 per-task executable override，例如 `npx tsx`、`pnpm tsx`、绝对路径。
* 可增加依赖检查命令，在任务保存时检测 runner 是否可用。
