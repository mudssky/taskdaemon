# Release Size Baseline

骨架阶段需要记录基础 release binary 体积，用于后续依赖增长对比。

## 构建命令

```bash
go build -trimpath -ldflags="-s -w" -o build/bin/taskdaemon ./cmd/taskdaemon
```

## 记录

| Date | GOOS/GOARCH | Command | Binary | Size |
|------|-------------|---------|--------|------|
| 2026-05-14 | windows/amd64 | `go build -trimpath -ldflags="-s -w" -o build/bin/taskdaemon.exe ./cmd/taskdaemon` | `build/bin/taskdaemon.exe` | 17,631,744 bytes (16.82 MiB) |
