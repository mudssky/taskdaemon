package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"entgo.io/ent/dialect/sql/schema"

	"taskdaemon/internal/data/ent/migrate"
)

// TableSpec 描述一张业务表的列清单。
type TableSpec struct {
	Name    string
	Columns []string
}

// BusinessTables 返回外键安全顺序的业务表目录（父表在前）。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []TableSpec: 有序业务表。
func BusinessTables() []TableSpec {
	// 顺序：admins → sessions → tasks → runs → notifications → audio_records
	order := []string{"admins", "sessions", "tasks", "runs", "notifications", "audio_records"}
	byName := map[string]*schema.Table{}
	for _, t := range migrate.Tables {
		byName[t.Name] = t
	}
	out := make([]TableSpec, 0, len(order))
	for _, name := range order {
		t, ok := byName[name]
		if !ok {
			continue
		}
		cols := make([]string, 0, len(t.Columns))
		for _, c := range t.Columns {
			cols = append(cols, c.Name)
		}
		out = append(out, TableSpec{Name: name, Columns: cols})
	}
	return out
}

// ExpectedFingerprint 基于 Ent migrate.Tables 计算应用期望 schema fingerprint。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: hex sha256。
func ExpectedFingerprint() string {
	parts := make([]string, 0, len(migrate.Tables)*8)
	names := make([]string, 0, len(migrate.Tables))
	byName := map[string]*schema.Table{}
	for _, t := range migrate.Tables {
		names = append(names, t.Name)
		byName[t.Name] = t
	}
	sort.Strings(names)
	for _, name := range names {
		t := byName[name]
		colParts := make([]string, 0, len(t.Columns))
		for _, c := range t.Columns {
			colParts = append(colParts, fmt.Sprintf("%s:%s", c.Name, c.Type.String()))
		}
		parts = append(parts, name+"{"+strings.Join(colParts, ",")+"}")
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

// TableByName 按名查找业务表规格。
//
// 参数:
//   - name: 表名。
//
// 返回值:
//   - TableSpec: 表规格。
//   - bool: 是否存在。
func TableByName(name string) (TableSpec, bool) {
	for _, t := range BusinessTables() {
		if t.Name == name {
			return t, true
		}
	}
	return TableSpec{}, false
}

// reverseTables 返回逆序表目录（清空时先子表）。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []TableSpec: 逆序业务表。
func reverseTables() []TableSpec {
	tables := BusinessTables()
	for i, j := 0, len(tables)-1; i < j; i, j = i+1, j-1 {
		tables[i], tables[j] = tables[j], tables[i]
	}
	return tables
}

// criticalSampleTables 参与抽样校验的关键表。
var criticalSampleTables = map[string]struct{}{
	"admins":        {},
	"tasks":         {},
	"notifications": {},
}
