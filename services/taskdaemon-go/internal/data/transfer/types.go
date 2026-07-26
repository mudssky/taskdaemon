package transfer

import (
	"io"
	"time"
)

// FormatVersion 是中间文件格式版本。
const FormatVersion = 1

// DefaultBatchSize 默认分批大小。
const DefaultBatchSize = 500

// ProgressFunc 报告迁移进度。
//
// 参数:
//   - table: 当前表名。
//   - done: 已处理行数。
//   - total: 该表总行数（未知时为 -1）。
//
// 返回值:
//   - 无。
type ProgressFunc func(table string, done, total int)

// Options 控制 export/import/transfer 行为。
type Options struct {
	// BatchSize 分批行数；<=0 时使用 DefaultBatchSize。
	BatchSize int
	// Force 允许在目标非空时继续（先清空业务表）。
	Force bool
	// DryRun 仅统计与打印，不写入目标/文件。
	DryRun bool
	// CheckpointPath 断点文件路径；空表示不分批落盘断点（小数据单事务）。
	CheckpointPath string
	// Progress 可选进度回调。
	Progress ProgressFunc
	// SampleSize 校验抽样行数；<=0 时默认 20。
	SampleSize int
	// SingleTxMaxRows 小于等于该行数时 import 使用单事务；0 表示默认 BatchSize*20。
	SingleTxMaxRows int
	// Writer 进度与提示输出（不含凭据）。
	Writer io.Writer
}

// TableCount 描述单表行数。
type TableCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Meta 是中间文件元信息。
type Meta struct {
	Type              string       `json:"type"`
	FormatVersion     int          `json:"formatVersion"`
	SchemaFingerprint string       `json:"schemaFingerprint"`
	ExportedAt        time.Time    `json:"exportedAt"`
	SourceDriver      string       `json:"sourceDriver"`
	Tables            []TableCount `json:"tables"`
}

// RowRecord 是中间文件中的一行数据。
type RowRecord struct {
	Type  string         `json:"type"`
	Table string         `json:"table"`
	Data  map[string]any `json:"data"`
}

// EOFRecord 标记文件结束。
type EOFRecord struct {
	Type     string `json:"type"`
	RowCount int    `json:"rowCount"`
}

// Checkpoint 记录分批导入断点。
type Checkpoint struct {
	Status          string    `json:"status"`
	Table           string    `json:"table,omitempty"`
	Offset          int       `json:"offset,omitempty"`
	CompletedTables []string  `json:"completedTables,omitempty"`
	Error           string    `json:"error,omitempty"`
	UpdatedAt       time.Time `json:"updatedAt"`
	TotalRows       int       `json:"totalRows,omitempty"`
	ImportedRows    int       `json:"importedRows,omitempty"`
}

// VerifyReport 是一致性校验报告。
type VerifyReport struct {
	OK      bool               `json:"ok"`
	Tables  []VerifyTableDiff  `json:"tables,omitempty"`
	Samples []VerifySampleDiff `json:"samples,omitempty"`
}

// VerifyTableDiff 描述表级计数差异。
type VerifyTableDiff struct {
	Table    string `json:"table"`
	Expected int    `json:"expected"`
	Actual   int    `json:"actual"`
}

// VerifySampleDiff 描述抽样内容差异。
type VerifySampleDiff struct {
	Table  string `json:"table"`
	ID     any    `json:"id"`
	Reason string `json:"reason"`
}

// batchSize 返回有效分批大小。
//
// 参数:
//   - opts: 选项。
//
// 返回值:
//   - int: 分批大小。
func batchSize(opts Options) int {
	if opts.BatchSize <= 0 {
		return DefaultBatchSize
	}
	return opts.BatchSize
}

// sampleSize 返回有效抽样大小。
//
// 参数:
//   - opts: 选项。
//
// 返回值:
//   - int: 抽样行数。
func sampleSize(opts Options) int {
	if opts.SampleSize <= 0 {
		return 20
	}
	return opts.SampleSize
}

// singleTxMax 返回单事务最大行数阈值。
//
// 参数:
//   - opts: 选项。
//
// 返回值:
//   - int: 阈值。
func singleTxMax(opts Options) int {
	if opts.SingleTxMaxRows > 0 {
		return opts.SingleTxMaxRows
	}
	return batchSize(opts) * 20
}
