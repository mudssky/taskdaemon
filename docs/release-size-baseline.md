# Release Size Baseline

骨架阶段需要记录基础 release binary 体积，用于后续依赖增长对比。

## 构建命令

```bash
cd services/taskdaemon-go
go build -trimpath -ldflags="-s -w" -o ../../build/bin/taskdaemon ./cmd/taskdaemon
```

## 记录

| Date | GOOS/GOARCH | Command | Binary | Size |
|------|-------------|---------|--------|------|
| 2026-05-14 | windows/amd64 | `go build -trimpath -ldflags="-s -w" -o build/bin/taskdaemon.exe ./cmd/taskdaemon` | `build/bin/taskdaemon.exe` | 17,631,744 bytes (16.82 MiB) |

> 说明：2026-05-14 的记录来自迁移前根 Go module 结构。迁移后从 `services/taskdaemon-go` 构建，并把产物输出到根 `build/bin/`。
