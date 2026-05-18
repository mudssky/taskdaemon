package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"

	"taskdaemon/internal/config"
)

// newFileHandler 创建带 lumberjack 轮转 writer 的 slog handler。
//
// 参数:
//   - cfg: 文件日志配置。
//   - opts: slog handler 选项。
//
// 返回值:
//   - slog.Handler: 文件日志 handler。
//   - error: 文件路径为空、目录创建失败或文件不可写时返回错误。
func newFileHandler(cfg config.LoggingFileConfig, opts *slog.HandlerOptions) (slog.Handler, io.Closer, error) {
	path := strings.TrimSpace(cfg.Path)
	if path == "" {
		return nil, nil, fmt.Errorf("logging file path is required when file output is enabled")
	}
	if err := ensureWritableLogFile(path); err != nil {
		return nil, nil, err
	}
	writer := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    cfg.MaxSizeMB,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAgeDays,
		Compress:   cfg.Compress,
	}
	return newHandler(writer, cfg.Format, opts), writer, nil
}

// ensureWritableLogFile 确认日志文件目录存在并且目标文件可写。
//
// 参数:
//   - path: 日志文件路径。
//
// 返回值:
//   - error: 创建目录或打开文件失败时返回错误。
func ensureWritableLogFile(path string) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create log directory %s: %w", dir, err)
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open log file %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close log file %s: %w", path, err)
	}
	return nil
}
