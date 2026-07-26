package transfer

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// LiveFingerprint 根据当前库业务表列集合计算 fingerprint（不比较方言类型细节）。
//
// 参数:
//   - ctx: 上下文。
//   - db: 数据库连接。
//   - driver: sqlite 或 postgres。
//
// 返回值:
//   - string: hex sha256。
//   - error: 查询失败时返回错误。
func LiveFingerprint(ctx context.Context, db *sql.DB, driver string) (string, error) {
	cols, err := listBusinessColumns(ctx, db, driver)
	if err != nil {
		return "", err
	}
	expected := BusinessTables()
	// 必须覆盖全部业务表
	for _, t := range expected {
		got, ok := cols[t.Name]
		if !ok {
			return "", newError(CodeSchemaMismatch, "missing business table on live database", nil).
				withDetails(map[string]any{"table": t.Name})
		}
		sort.Strings(got)
		want := append([]string(nil), t.Columns...)
		sort.Strings(want)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			return "", newError(CodeSchemaMismatch, "column set mismatch", nil).
				withDetails(map[string]any{"table": t.Name, "want": want, "got": got})
		}
	}
	// 与 ExpectedFingerprint 同源：表+列名（类型用 live 不可靠，使用期望类型串）
	return ExpectedFingerprint(), nil
}

// EnsureSchemaMatch 校验 live 库与应用期望 schema 一致。
//
// 参数:
//   - ctx: 上下文。
//   - db: 数据库连接。
//   - driver: 方言。
//
// 返回值:
//   - string: fingerprint。
//   - error: 不一致或查询失败。
func EnsureSchemaMatch(ctx context.Context, db *sql.DB, driver string) (string, error) {
	fp, err := LiveFingerprint(ctx, db, driver)
	if err != nil {
		return "", err
	}
	want := ExpectedFingerprint()
	if fp != want {
		return "", newError(CodeSchemaMismatch, "schema fingerprint mismatch", nil).
			withDetails(map[string]any{"want": want, "got": fp})
	}
	return fp, nil
}

// listBusinessColumns 列出业务表列名。
//
// 参数:
//   - ctx: 上下文。
//   - db: 数据库连接。
//   - driver: 方言。
//
// 返回值:
//   - map[string][]string: 表 → 列名列表。
//   - error: 查询失败。
func listBusinessColumns(ctx context.Context, db *sql.DB, driver string) (map[string][]string, error) {
	business := make(map[string]struct{})
	for _, t := range BusinessTables() {
		business[t.Name] = struct{}{}
	}
	out := map[string][]string{}
	switch strings.ToLower(driver) {
	case "sqlite":
		rows, err := db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`)
		if err != nil {
			return nil, newError(CodeConnectFailed, "list sqlite tables", err)
		}
		defer rows.Close()
		var tables []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, err
			}
			if _, ok := business[name]; ok {
				tables = append(tables, name)
			}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		for _, name := range tables {
			info, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", quoteIdent(driver, name)))
			if err != nil {
				return nil, err
			}
			var cols []string
			for info.Next() {
				var cid int
				var cname, ctype string
				var notnull, pk int
				var dflt sql.NullString
				if err := info.Scan(&cid, &cname, &ctype, &notnull, &dflt, &pk); err != nil {
					info.Close()
					return nil, err
				}
				cols = append(cols, cname)
			}
			info.Close()
			out[name] = cols
		}
		return out, nil
	case "postgres":
		rows, err := db.QueryContext(ctx, `
			SELECT table_name, column_name
			FROM information_schema.columns
			WHERE table_schema = 'public'
			ORDER BY table_name, ordinal_position`)
		if err != nil {
			return nil, newError(CodeConnectFailed, "list postgres columns", err)
		}
		defer rows.Close()
		for rows.Next() {
			var table, col string
			if err := rows.Scan(&table, &col); err != nil {
				return nil, err
			}
			if _, ok := business[table]; !ok {
				continue
			}
			out[table] = append(out[table], col)
		}
		return out, rows.Err()
	default:
		return nil, newError(CodeInvalidInput, "unsupported driver for fingerprint", nil).
			withDetails(map[string]any{"driver": driver})
	}
}

// quoteIdent 按方言引用标识符。
//
// 参数:
//   - driver: 方言。
//   - name: 标识符。
//
// 返回值:
//   - string: 引用后的标识符。
func quoteIdent(driver, name string) string {
	// 两端仅允许安全标识符
	if strings.ContainsAny(name, "\"`; \t\n") {
		name = strings.ReplaceAll(name, `"`, "")
	}
	return `"` + name + `"`
}

// hashFingerprintParts 辅助哈希。
//
// 参数:
//   - parts: 参与哈希的字符串。
//
// 返回值:
//   - string: hex sha256。
func hashFingerprintParts(parts []string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}
