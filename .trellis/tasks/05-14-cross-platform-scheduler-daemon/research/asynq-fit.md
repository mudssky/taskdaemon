# Asynq 适配性调研

## 背景

用户提出：本项目的用户定时任务与分布式任务队列是不同场景，因此第一版不应默认引入 Asynq。

## 资料来源

* Context7: `/hibiken/asynq`

## 关键结论

* Asynq 是 Redis-backed 的分布式后台任务队列。
* 核心模型是 client enqueue task，worker 异步处理 task。
* 支持至少一次执行、自动重试、队列优先级、去重、超时、deadline、周期任务等能力。
* Scheduler 可以按 cron 或 interval 把任务 enqueue 到 Redis 队列。

## 与本项目目标的匹配度

本项目第一阶段目标：

* 本机/单节点守护进程。
* 低资源占用。
* 单二进制发布。
* 用户脚本/命令的定时执行。
* SQLite/PostgreSQL 保存配置和执行历史。

Asynq 的问题：

* 引入 Redis 运行时依赖，不符合单二进制和低占用目标。
* 分布式队列语义会增加 worker、queue、retry、at-least-once 等复杂度。
* 对用户本机脚本调度来说能力过重。

## 决策

第一版不使用 Asynq 作为调度核心。采用进程内 cron 调度 + 本地执行历史持久化。

## 后续可能性

如果未来出现分布式任务、集中队列、跨节点 worker、任务削峰等需求，可重新评估 Asynq、NATS、Temporal 等后台任务/工作流方案。
