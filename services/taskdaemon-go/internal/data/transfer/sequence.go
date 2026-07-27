package transfer

import (
	"context"
	"fmt"
	"strings"

	"taskdaemon/internal/data"
)

// ResetSequences 在导入后复位自增序列，避免后续插入主键冲突。
//
// 参数:
//   - ctx: 上下文。
//   - store: 目标库。
//
// 返回值:
//   - error: 复位失败。
func ResetSequences(ctx context.Context, store *data.Store) error {
	if store == nil || store.SQL() == nil {
		return newError(CodeConnectFailed, "store is not open", nil)
	}
	driver := store.Driver()
	db := store.SQL()
	for _, t := range BusinessTables() {
		var maxID sqlNullInt64
		q := fmt.Sprintf(`SELECT COALESCE(MAX(%s), 0) FROM %s`, quoteIdent(driver, "id"), quoteIdent(driver, t.Name))
		if err := db.QueryRowContext(ctx, q).Scan(&maxID); err != nil {
			return newError(CodeImportFailed, "select max id for "+t.Name, err)
		}
		next := maxID.Int64
		if next < 0 {
			next = 0
		}
		switch strings.ToLower(driver) {
		case "postgres":
			// is_called=true 表示下一次 nextval 返回 max+1；空表时设为 1 且 is_called=false
			if maxID.Int64 == 0 {
				seqQ := fmt.Sprintf(`SELECT setval(pg_get_serial_sequence('%s', 'id'), 1, false)`, t.Name)
				if _, err := db.ExecContext(ctx, seqQ); err != nil {
					// 某些表可能没有 serial sequence 绑定；忽略不存在
					if !strings.Contains(err.Error(), "null value") && !strings.Contains(strings.ToLower(err.Error()), "does not exist") {
						return newError(CodePermission, "reset sequence "+t.Name, err)
					}
				}
				continue
			}
			seqQ := fmt.Sprintf(`SELECT setval(pg_get_serial_sequence('%s', 'id'), %d, true)`, t.Name, maxID.Int64)
			if _, err := db.ExecContext(ctx, seqQ); err != nil {
				return newError(CodePermission, "reset sequence "+t.Name, err)
			}
		case "sqlite":
			// 维护 sqlite_sequence；空表删除条目
			if maxID.Int64 == 0 {
				_, _ = db.ExecContext(ctx, `DELETE FROM sqlite_sequence WHERE name = ?`, t.Name)
				continue
			}
			// 尝试更新；无则插入
			res, err := db.ExecContext(ctx, `UPDATE sqlite_sequence SET seq = ? WHERE name = ?`, maxID.Int64, t.Name)
			if err != nil {
				// 某些 SQLite 编译未启用 AUTOINCREMENT 时无 sqlite_sequence
				if strings.Contains(err.Error(), "no such table") {
					continue
				}
				return newError(CodePermission, "update sqlite_sequence "+t.Name, err)
			}
			affected, _ := res.RowsAffected()
			if affected == 0 {
				_, err = db.ExecContext(ctx, `INSERT INTO sqlite_sequence(name, seq) VALUES(?, ?)`, t.Name, maxID.Int64)
				if err != nil && !strings.Contains(err.Error(), "no such table") {
					return newError(CodePermission, "insert sqlite_sequence "+t.Name, err)
				}
			}
		}
	}
	return nil
}

// sqlNullInt64 简化扫描。
type sqlNullInt64 struct {
	Int64 int64
	Valid bool
}

// Scan 实现 sql.Scanner。
//
// 参数:
//   - value: 驱动值。
//
// 返回值:
//   - error: 转换失败。
func (n *sqlNullInt64) Scan(value any) error {
	if value == nil {
		n.Int64, n.Valid = 0, false
		return nil
	}
	switch t := value.(type) {
	case int64:
		n.Int64, n.Valid = t, true
	case int32:
		n.Int64, n.Valid = int64(t), true
	case int:
		n.Int64, n.Valid = int64(t), true
	case []byte:
		var v int64
		_, err := fmt.Sscan(string(t), &v)
		if err != nil {
			return err
		}
		n.Int64, n.Valid = v, true
	default:
		var v int64
		_, err := fmt.Sscan(fmt.Sprint(t), &v)
		if err != nil {
			return err
		}
		n.Int64, n.Valid = v, true
	}
	return nil
}
