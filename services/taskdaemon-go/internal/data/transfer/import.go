package transfer

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"taskdaemon/internal/data"
)

// Import 从中间文件导入到目标库。
//
// 参数:
//   - ctx: 上下文。
//   - store: 目标库（应已 migrate）。
//   - inputPath: .tdxfer 路径。
//   - opts: 导入选项。
//
// 返回值:
//   - *VerifyReport: 校验报告。
//   - error: 失败时返回错误。
func Import(ctx context.Context, store *data.Store, inputPath string, opts Options) (*VerifyReport, error) {
	if store == nil || store.SQL() == nil {
		return nil, newError(CodeConnectFailed, "target store is not open", nil)
	}
	if strings.TrimSpace(inputPath) == "" {
		return nil, newError(CodeInvalidInput, "input path is required", nil)
	}
	printBackupHint(opts.Writer)

	fp, err := EnsureSchemaMatch(ctx, store.SQL(), store.Driver())
	if err != nil {
		return nil, err
	}

	file, err := os.Open(inputPath)
	if err != nil {
		return nil, newError(CodeInvalidInput, "open input file", err)
	}
	defer file.Close()

	meta, scanner, err := DecodeMeta(file)
	if err != nil {
		return nil, err
	}
	if meta.SchemaFingerprint != fp {
		return nil, newError(CodeSchemaMismatch, "export fingerprint does not match target schema", nil).
			withDetails(map[string]any{"export": meta.SchemaFingerprint, "target": fp})
	}

	total := sumCounts(meta.Tables)
	if opts.DryRun {
		printDryRun(opts.Writer, "import", meta.Tables)
		return &VerifyReport{OK: true}, nil
	}

	nonEmpty, err := anyBusinessRows(ctx, store.SQL(), store.Driver())
	if err != nil {
		return nil, err
	}
	if nonEmpty && !opts.Force {
		return nil, newError(CodeTargetNotEmpty, "target database is not empty; pass --force to confirm overwrite", nil)
	}

	useSingleTx := opts.CheckpointPath == "" && total <= singleTxMax(opts)
	if useSingleTx {
		if err := importStreamSingleTx(ctx, store, scanner, meta, opts); err != nil {
			return nil, err
		}
	} else {
		if err := importStreamBatched(ctx, store, scanner, meta, opts); err != nil {
			return nil, err
		}
	}

	if err := ResetSequences(ctx, store); err != nil {
		return nil, err
	}

	report, err := VerifyFromMeta(ctx, store, meta, opts)
	if err != nil {
		return nil, err
	}
	if !report.OK {
		return report, newError(CodeVerifyFailed, "post-import verification failed", nil).
			withDetails(map[string]any{"report": report})
	}
	return report, nil
}

// importStreamSingleTx 流式单事务导入。
//
// 参数:
//   - ctx: 上下文。
//   - store: 目标库。
//   - scanner: 已消费 meta 的 scanner。
//   - meta: 元信息。
//   - opts: 选项。
//
// 返回值:
//   - error: 失败。
func importStreamSingleTx(ctx context.Context, store *data.Store, scanner *bufio.Scanner, meta *Meta, opts Options) error {
	tx, err := store.SQL().BeginTx(ctx, nil)
	if err != nil {
		return newError(CodeImportFailed, "begin transaction", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := clearBusinessTablesTx(ctx, tx, store.Driver()); err != nil {
		return err
	}

	bs := batchSize(opts)
	var (
		current string
		batch   []map[string]any
		done    int
	)
	flush := func() error {
		if current == "" || len(batch) == 0 {
			return nil
		}
		spec, ok := TableByName(current)
		if !ok {
			return newError(CodeInvalidInput, "unknown table", nil).withDetails(map[string]any{"table": current})
		}
		if err := insertRowsTx(ctx, tx, store.Driver(), spec, batch, opts); err != nil {
			return err
		}
		done += len(batch)
		reportProgress(opts, current, done, countFor(meta.Tables, current))
		batch = batch[:0]
		return nil
	}

	if err := forEachRow(scanner, func(table string, row map[string]any) error {
		if table != current {
			if err := flush(); err != nil {
				return err
			}
			current = table
			done = 0
		}
		batch = append(batch, row)
		if len(batch) >= bs {
			return flush()
		}
		return nil
	}); err != nil {
		return err
	}
	if err := flush(); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return newError(CodeImportFailed, "commit transaction", err)
	}
	return nil
}

// importStreamBatched 流式分批提交并写 checkpoint。
//
// 参数:
//   - ctx: 上下文。
//   - store: 目标库。
//   - scanner: scanner。
//   - meta: 元信息。
//   - opts: 选项。
//
// 返回值:
//   - error: 失败。
func importStreamBatched(ctx context.Context, store *data.Store, scanner *bufio.Scanner, meta *Meta, opts Options) error {
	cp, err := loadCheckpoint(opts.CheckpointPath)
	if err != nil {
		return err
	}
	if cp != nil && cp.Status == "completed" && !opts.Force {
		return newError(CodeCheckpointInvalid, "checkpoint already completed; use --force to re-import", nil)
	}

	completed := map[string]struct{}{}
	skipUntilTable := ""
	skipUntilOffset := 0
	if cp != nil && cp.Status == "in_progress" {
		for _, name := range cp.CompletedTables {
			completed[name] = struct{}{}
		}
		skipUntilTable = cp.Table
		skipUntilOffset = cp.Offset
	} else {
		if err := clearBusinessTables(ctx, store.SQL(), store.Driver()); err != nil {
			return err
		}
	}

	bs := batchSize(opts)
	var (
		current  string
		batch    []map[string]any
		offset   int
		imported int
	)
	if cp != nil {
		imported = cp.ImportedRows
	}

	flush := func() error {
		if current == "" || len(batch) == 0 {
			return nil
		}
		if _, done := completed[current]; done {
			batch = batch[:0]
			return nil
		}
		spec, ok := TableByName(current)
		if !ok {
			return newError(CodeInvalidInput, "unknown table", nil).withDetails(map[string]any{"table": current})
		}
		tx, err := store.SQL().BeginTx(ctx, nil)
		if err != nil {
			_ = saveCheckpoint(opts.CheckpointPath, Checkpoint{
				Status: "failed", Table: current, Offset: offset, CompletedTables: keys(completed),
				Error: err.Error(), UpdatedAt: time.Now().UTC(), ImportedRows: imported,
			})
			return newError(CodeImportFailed, "begin batch tx", err)
		}
		if err := insertRowsTx(ctx, tx, store.Driver(), spec, batch, opts); err != nil {
			_ = tx.Rollback()
			_ = saveCheckpoint(opts.CheckpointPath, Checkpoint{
				Status: "failed", Table: current, Offset: offset, CompletedTables: keys(completed),
				Error: err.Error(), UpdatedAt: time.Now().UTC(), ImportedRows: imported,
			})
			return err
		}
		if err := tx.Commit(); err != nil {
			return newError(CodeImportFailed, "commit batch", err)
		}
		n := len(batch)
		offset += n
		imported += n
		reportProgress(opts, current, offset, countFor(meta.Tables, current))
		batch = batch[:0]
		if opts.CheckpointPath != "" {
			return saveCheckpoint(opts.CheckpointPath, Checkpoint{
				Status: "in_progress", Table: current, Offset: offset, CompletedTables: keys(completed),
				UpdatedAt: time.Now().UTC(), TotalRows: sumCounts(meta.Tables), ImportedRows: imported,
			})
		}
		return nil
	}

	tableRowIndex := map[string]int{}
	if err := forEachRow(scanner, func(table string, row map[string]any) error {
		idx := tableRowIndex[table]
		tableRowIndex[table] = idx + 1

		// 跳过已完成表
		if _, done := completed[table]; done {
			return nil
		}
		// 跳过断点前的行
		if skipUntilTable != "" {
			if table != skipUntilTable {
				// 仍在更早的表：应已在 completed；若否，跳过
				return nil
			}
			if idx < skipUntilOffset {
				return nil
			}
			// 到达续跑点后清除 skip 标记
			if idx == skipUntilOffset {
				skipUntilTable = ""
			}
		}

		if table != current {
			if err := flush(); err != nil {
				return err
			}
			if current != "" && current != table {
				// 上一表结束
				if _, done := completed[current]; !done && (skipUntilTable == "" || current != skipUntilTable) {
					// 仅当已完整处理该表时标记 completed：offset 达到 meta count
					if offset >= countFor(meta.Tables, current) || countFor(meta.Tables, current) == 0 {
						completed[current] = struct{}{}
					}
				}
			}
			current = table
			offset = idx
			// 续跑时 offset 从 skipUntilOffset 开始
			if cp != nil && cp.Status == "in_progress" && table == cp.Table && idx == cp.Offset {
				offset = cp.Offset
			}
		}
		batch = append(batch, row)
		if len(batch) >= bs {
			return flush()
		}
		return nil
	}); err != nil {
		return err
	}
	if err := flush(); err != nil {
		return err
	}
	if current != "" {
		completed[current] = struct{}{}
	}
	if opts.CheckpointPath != "" {
		return saveCheckpoint(opts.CheckpointPath, Checkpoint{
			Status: "completed", CompletedTables: keys(completed),
			UpdatedAt: time.Now().UTC(), TotalRows: sumCounts(meta.Tables), ImportedRows: imported,
		})
	}
	return nil
}

// forEachRow 遍历 scanner 中的 row 记录直至 eof。
//
// 参数:
//   - scanner: NDJSON scanner。
//   - fn: 每行回调。
//
// 返回值:
//   - error: 解析或回调失败。
func forEachRow(scanner *bufio.Scanner, fn func(table string, row map[string]any) error) error {
	for scanner.Scan() {
		line := scanner.Bytes()
		var head struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(line, &head); err != nil {
			return newError(CodeInvalidInput, "parse record", err)
		}
		switch head.Type {
		case "row":
			var rec RowRecord
			if err := json.Unmarshal(line, &rec); err != nil {
				return newError(CodeInvalidInput, "parse row", err)
			}
			if _, ok := TableByName(rec.Table); !ok {
				return newError(CodeInvalidInput, "unknown table in export", nil).
					withDetails(map[string]any{"table": rec.Table})
			}
			if err := fn(rec.Table, rec.Data); err != nil {
				return err
			}
		case "eof":
			return scanner.Err()
		case "meta":
			return newError(CodeInvalidInput, "duplicate meta record", nil)
		default:
			return newError(CodeInvalidInput, "unknown record type", nil).
				withDetails(map[string]any{"type": head.Type})
		}
	}
	if err := scanner.Err(); err != nil {
		return newError(CodeInvalidInput, "read rows", err)
	}
	return nil
}

// insertRowsTx 在事务中插入多行（保留主键）。
//
// 参数:
//   - ctx: 上下文。
//   - tx: 事务。
//   - driver: 方言。
//   - table: 表规格。
//   - rows: 行数据。
//   - opts: 选项（保留扩展）。
//
// 返回值:
//   - error: 插入失败。
func insertRowsTx(ctx context.Context, tx *sql.Tx, driver string, table TableSpec, rows []map[string]any, opts Options) error {
	_ = opts
	if len(rows) == 0 {
		return nil
	}
	cols := table.Columns
	quoted := make([]string, len(cols))
	for i, c := range cols {
		quoted[i] = quoteIdent(driver, c)
	}
	placeholders := make([]string, len(cols))
	for i := range cols {
		if driver == "postgres" {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		} else {
			placeholders[i] = "?"
		}
	}
	q := fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s)`,
		quoteIdent(driver, table.Name),
		strings.Join(quoted, ", "),
		strings.Join(placeholders, ", "),
	)
	stmt, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return newError(CodePermission, "prepare insert "+table.Name, err)
	}
	defer stmt.Close()

	for _, row := range rows {
		args := make([]any, len(cols))
		for i, c := range cols {
			v, err := bindValue(driver, c, row[c])
			if err != nil {
				return newError(CodeImportFailed, "bind "+table.Name+"."+c, err)
			}
			args[i] = v
		}
		if _, err := stmt.ExecContext(ctx, args...); err != nil {
			return newError(CodeImportFailed, "insert "+table.Name, err)
		}
	}
	return nil
}

// clearBusinessTables 清空业务表（逆序）。
//
// 参数:
//   - ctx: 上下文。
//   - db: 连接。
//   - driver: 方言。
//
// 返回值:
//   - error: 失败。
func clearBusinessTables(ctx context.Context, db *sql.DB, driver string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return newError(CodeImportFailed, "begin clear", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := clearBusinessTablesTx(ctx, tx, driver); err != nil {
		return err
	}
	return tx.Commit()
}

// clearBusinessTablesTx 在事务内清空业务表。
//
// 参数:
//   - ctx: 上下文。
//   - tx: 事务。
//   - driver: 方言。
//
// 返回值:
//   - error: 失败。
func clearBusinessTablesTx(ctx context.Context, tx *sql.Tx, driver string) error {
	for _, t := range reverseTables() {
		q := fmt.Sprintf(`DELETE FROM %s`, quoteIdent(driver, t.Name))
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return newError(CodePermission, "delete "+t.Name, err)
		}
	}
	return nil
}

// anyBusinessRows 判断目标是否已有业务数据。
//
// 参数:
//   - ctx: 上下文。
//   - db: 连接。
//   - driver: 方言。
//
// 返回值:
//   - bool: 是否非空。
//   - error: 查询失败。
func anyBusinessRows(ctx context.Context, db *sql.DB, driver string) (bool, error) {
	for _, t := range BusinessTables() {
		n, err := countTable(ctx, db, driver, t.Name)
		if err != nil {
			return false, err
		}
		if n > 0 {
			return true, nil
		}
	}
	return false, nil
}

// keys 返回 map 键切片。
func keys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// sumCounts 汇总行数。
func sumCounts(tables []TableCount) int {
	n := 0
	for _, t := range tables {
		n += t.Count
	}
	return n
}
