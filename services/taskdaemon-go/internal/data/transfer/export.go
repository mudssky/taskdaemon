package transfer

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"taskdaemon/internal/data"
)

// Export 将源库业务表导出到中间文件。
//
// 参数:
//   - ctx: 上下文。
//   - store: 已打开并 migrate 的源库。
//   - outputPath: 输出 .tdxfer 路径。
//   - opts: 导出选项。
//
// 返回值:
//   - *Meta: 导出元信息。
//   - error: 失败时返回错误。
func Export(ctx context.Context, store *data.Store, outputPath string, opts Options) (*Meta, error) {
	if store == nil || store.SQL() == nil {
		return nil, newError(CodeConnectFailed, "source store is not open", nil)
	}
	if strings.TrimSpace(outputPath) == "" && !opts.DryRun {
		return nil, newError(CodeInvalidInput, "output path is required", nil)
	}
	printBackupHint(opts.Writer)

	fp, err := EnsureSchemaMatch(ctx, store.SQL(), store.Driver())
	if err != nil {
		return nil, err
	}

	tables := BusinessTables()
	counts := make([]TableCount, 0, len(tables))
	totalRows := 0
	for _, t := range tables {
		n, err := countTable(ctx, store.SQL(), store.Driver(), t.Name)
		if err != nil {
			return nil, err
		}
		counts = append(counts, TableCount{Name: t.Name, Count: n})
		totalRows += n
		reportProgress(opts, t.Name, 0, n)
	}

	meta := &Meta{
		Type:              "meta",
		FormatVersion:     FormatVersion,
		SchemaFingerprint: fp,
		ExportedAt:        time.Now().UTC(),
		SourceDriver:      store.Driver(),
		Tables:            counts,
	}
	if opts.DryRun {
		printDryRun(opts.Writer, "export", counts)
		return meta, nil
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return nil, newError(CodeInvalidInput, "create output file", err)
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	if err := enc.Encode(meta); err != nil {
		return nil, newError(CodeImportFailed, "write meta", err)
	}

	written := 0
	bs := batchSize(opts)
	for _, t := range tables {
		offset := 0
		for {
			rows, err := fetchRows(ctx, store.SQL(), store.Driver(), t, offset, bs)
			if err != nil {
				return nil, err
			}
			if len(rows) == 0 {
				break
			}
			for _, row := range rows {
				rec := RowRecord{Type: "row", Table: t.Name, Data: row}
				if err := enc.Encode(rec); err != nil {
					return nil, newError(CodeImportFailed, "write row", err)
				}
				written++
			}
			offset += len(rows)
			reportProgress(opts, t.Name, offset, countFor(counts, t.Name))
			if len(rows) < bs {
				break
			}
		}
	}
	if err := enc.Encode(EOFRecord{Type: "eof", RowCount: written}); err != nil {
		return nil, newError(CodeImportFailed, "write eof", err)
	}
	return meta, nil
}

// countTable 统计表行数。
//
// 参数:
//   - ctx: 上下文。
//   - db: 连接。
//   - driver: 方言。
//   - table: 表名。
//
// 返回值:
//   - int: 行数。
//   - error: 查询失败。
func countTable(ctx context.Context, db *sql.DB, driver, table string) (int, error) {
	q := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, quoteIdent(driver, table))
	var n int
	if err := db.QueryRowContext(ctx, q).Scan(&n); err != nil {
		return 0, newError(CodeConnectFailed, "count table "+table, err)
	}
	return n, nil
}

// fetchRows 分页读取表行。
//
// 参数:
//   - ctx: 上下文。
//   - db: 连接。
//   - driver: 方言。
//   - table: 表规格。
//   - offset: 偏移。
//   - limit: 限制。
//
// 返回值:
//   - []map[string]any: 行列表。
//   - error: 查询失败。
func fetchRows(ctx context.Context, db *sql.DB, driver string, table TableSpec, offset, limit int) ([]map[string]any, error) {
	cols := make([]string, len(table.Columns))
	for i, c := range table.Columns {
		cols[i] = quoteIdent(driver, c)
	}
	q := fmt.Sprintf(
		`SELECT %s FROM %s ORDER BY %s LIMIT %d OFFSET %d`,
		strings.Join(cols, ", "),
		quoteIdent(driver, table.Name),
		quoteIdent(driver, "id"),
		limit,
		offset,
	)
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, newError(CodeConnectFailed, "select "+table.Name, err)
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		dest, raw := scanDest(len(table.Columns))
		if err := rows.Scan(dest...); err != nil {
			return nil, newError(CodeImportFailed, "scan "+table.Name, err)
		}
		// 解引用 *any
		values := make([]any, len(raw))
		for i := range raw {
			values[i] = raw[i]
		}
		row, err := normalizeRow(table.Columns, values)
		if err != nil {
			return nil, newError(CodeImportFailed, "normalize "+table.Name, err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// countFor 查表计数。
func countFor(counts []TableCount, name string) int {
	for _, c := range counts {
		if c.Name == name {
			return c.Count
		}
	}
	return -1
}

// reportProgress 调用进度回调。
func reportProgress(opts Options, table string, done, total int) {
	if opts.Progress != nil {
		opts.Progress(table, done, total)
	}
	if opts.Writer != nil && total >= 0 {
		fmt.Fprintf(opts.Writer, "progress table=%s done=%d total=%d\n", table, done, total)
	}
}

// printBackupHint 打印备份与停服提示。
func printBackupHint(w io.Writer) {
	if w == nil {
		return
	}
	fmt.Fprintln(w, "WARNING: Back up source and target databases before migration.")
	fmt.Fprintln(w, "WARNING: Stop the taskdaemon service during migration to avoid concurrent writes.")
}

// printDryRun 打印 dry-run 统计。
func printDryRun(w io.Writer, op string, counts []TableCount) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, "dry-run %s:\n", op)
	total := 0
	for _, c := range counts {
		fmt.Fprintf(w, "  - %s: %d\n", c.Name, c.Count)
		total += c.Count
	}
	fmt.Fprintf(w, "total rows: %d\n", total)
}

// DecodeMeta 从 reader 读取第一行 meta。
//
// 参数:
//   - r: 输入。
//
// 返回值:
//   - *Meta: 元信息。
//   - *bufio.Scanner: 已消费首行的 scanner。
//   - error: 解析失败。
func DecodeMeta(r io.Reader) (*Meta, *bufio.Scanner, error) {
	sc := bufio.NewScanner(r)
	// 大行：JSON 可能较大
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 16*1024*1024)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return nil, nil, newError(CodeInvalidInput, "read meta", err)
		}
		return nil, nil, newError(CodeInvalidInput, "empty transfer file", nil)
	}
	var meta Meta
	if err := json.Unmarshal(sc.Bytes(), &meta); err != nil {
		return nil, nil, newError(CodeInvalidInput, "parse meta", err)
	}
	if meta.Type != "meta" {
		return nil, nil, newError(CodeInvalidInput, "first record must be meta", nil)
	}
	if meta.FormatVersion != FormatVersion {
		return nil, nil, newError(CodeInvalidInput, "unsupported format version", nil).
			withDetails(map[string]any{"version": meta.FormatVersion})
	}
	return &meta, sc, nil
}
