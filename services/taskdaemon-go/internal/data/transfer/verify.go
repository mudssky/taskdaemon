package transfer

import (
	"context"
	"fmt"

	"taskdaemon/internal/data"
)

// VerifyFromMeta 按 meta 中的表计数与关键表抽样校验目标库。
//
// 参数:
//   - ctx: 上下文。
//   - store: 目标库。
//   - meta: 源侧元信息（含期望计数）。
//   - opts: 选项（抽样大小）。
//
// 返回值:
//   - *VerifyReport: 校验报告。
//   - error: 查询失败。
func VerifyFromMeta(ctx context.Context, store *data.Store, meta *Meta, opts Options) (*VerifyReport, error) {
	report := &VerifyReport{OK: true}
	if store == nil || store.SQL() == nil {
		return nil, newError(CodeConnectFailed, "store is not open", nil)
	}
	// 计数
	for _, tc := range meta.Tables {
		actual, err := countTable(ctx, store.SQL(), store.Driver(), tc.Name)
		if err != nil {
			return nil, err
		}
		if actual != tc.Count {
			report.OK = false
			report.Tables = append(report.Tables, VerifyTableDiff{
				Table:    tc.Name,
				Expected: tc.Count,
				Actual:   actual,
			})
		}
	}
	// 抽样：从 meta 无法取源行时，仅校验目标可查询；Transfer 路径另有源侧校验入口
	// 这里对关键表按 id 抽样读取，确保行可读且 id 连续存在（与 count 一致时抽样应成功）
	limit := sampleSize(opts)
	for _, t := range BusinessTables() {
		if _, ok := criticalSampleTables[t.Name]; !ok {
			continue
		}
		rows, err := fetchRows(ctx, store.SQL(), store.Driver(), t, 0, limit)
		if err != nil {
			return nil, err
		}
		// 若 count 匹配但抽不到行（count=0）则跳过
		expectedCount := countFor(meta.Tables, t.Name)
		if expectedCount > 0 && len(rows) == 0 {
			report.OK = false
			report.Samples = append(report.Samples, VerifySampleDiff{
				Table:  t.Name,
				Reason: "expected rows but sample empty",
			})
			continue
		}
		for _, row := range rows {
			if row["id"] == nil {
				report.OK = false
				report.Samples = append(report.Samples, VerifySampleDiff{
					Table:  t.Name,
					Reason: "missing id in sample row",
				})
			}
		}
	}
	return report, nil
}

// VerifyStores 对比源库与目标库计数与关键表抽样内容。
//
// 参数:
//   - ctx: 上下文。
//   - source: 源库。
//   - target: 目标库。
//   - opts: 选项。
//
// 返回值:
//   - *VerifyReport: 报告。
//   - error: 失败。
func VerifyStores(ctx context.Context, source, target *data.Store, opts Options) (*VerifyReport, error) {
	report := &VerifyReport{OK: true}
	limit := sampleSize(opts)
	for _, t := range BusinessTables() {
		sc, err := countTable(ctx, source.SQL(), source.Driver(), t.Name)
		if err != nil {
			return nil, err
		}
		tc, err := countTable(ctx, target.SQL(), target.Driver(), t.Name)
		if err != nil {
			return nil, err
		}
		if sc != tc {
			report.OK = false
			report.Tables = append(report.Tables, VerifyTableDiff{Table: t.Name, Expected: sc, Actual: tc})
		}
		if _, ok := criticalSampleTables[t.Name]; !ok {
			continue
		}
		srcRows, err := fetchRows(ctx, source.SQL(), source.Driver(), t, 0, limit)
		if err != nil {
			return nil, err
		}
		dstRows, err := fetchRows(ctx, target.SQL(), target.Driver(), t, 0, limit)
		if err != nil {
			return nil, err
		}
		if len(srcRows) != len(dstRows) {
			report.OK = false
			report.Samples = append(report.Samples, VerifySampleDiff{
				Table:  t.Name,
				Reason: fmt.Sprintf("sample size source=%d target=%d", len(srcRows), len(dstRows)),
			})
			continue
		}
		// 按 id 建索引比较
		dstByID := map[string]map[string]any{}
		for _, r := range dstRows {
			dstByID[formatID(r["id"])] = r
		}
		for _, r := range srcRows {
			id := formatID(r["id"])
			dr, ok := dstByID[id]
			if !ok {
				report.OK = false
				report.Samples = append(report.Samples, VerifySampleDiff{Table: t.Name, ID: r["id"], Reason: "missing on target"})
				continue
			}
			if !rowEqual(r, dr) {
				report.OK = false
				report.Samples = append(report.Samples, VerifySampleDiff{Table: t.Name, ID: r["id"], Reason: "content mismatch"})
			}
		}
	}
	return report, nil
}
