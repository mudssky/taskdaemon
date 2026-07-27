package transfer_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"taskdaemon/internal/config"
	"taskdaemon/internal/data"
	"taskdaemon/internal/data/ent/migrate"
	"taskdaemon/internal/data/transfer"
)

// openTempSQLite 打开临时 SQLite 并 migrate。
//
// 参数:
//   - t: 测试上下文。
//   - name: 文件名。
//
// 返回值:
//   - *data.Store: 已打开的 store。
func openTempSQLite(t *testing.T, name string) *data.Store {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), name)
	store, err := data.Open(ctx, config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    "file:" + path + "?_fk=1",
	})
	require.NoError(t, err)
	require.NoError(t, store.Migrate(ctx))
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// seedSampleData 写入覆盖类型映射的样例数据。
//
// 参数:
//   - t: 测试上下文。
//   - store: 目标库。
//
// 返回值:
//   - 无。
func seedSampleData(t *testing.T, store *data.Store) {
	t.Helper()
	ctx := context.Background()
	db := store.SQL()
	ts := time.Date(2026, 7, 27, 12, 30, 0, 123456789, time.UTC)

	_, err := db.ExecContext(ctx, `
INSERT INTO admins (id, singleton_key, username, password_hash, active, created_at, updated_at)
VALUES (7, 'singleton', 'admin', 'hash', 1, ?, ?)`, ts, ts)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
INSERT INTO sessions (id, token_hash, csrf_token_hash, expires_at, user_agent, ip, created_at, updated_at, admin_sessions)
VALUES (3, 'tok', 'csrf', ?, 'ua', '127.0.0.1', ?, ?, 7)`, ts.Add(time.Hour), ts, ts)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
INSERT INTO tasks (id, name, description, enabled, cron_expression, timezone, runner_type, runner_config, timeout_seconds, overlap_policy, created_at, updated_at)
VALUES (11, 'demo', 'desc', 1, '*/5 * * * *', 'UTC', 'shell', ?, 60, 'skip', ?, ?)`,
		`{"command":"echo","args":["hi"]}`, ts, ts)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
INSERT INTO runs (id, trigger, status, exit_code, started_at, finished_at, duration_ms, error_summary, stdout, stderr, log_archive_status, log_size_bytes, log_write_failed, created_at, updated_at, task_runs)
VALUES (21, 'manual', 'success', 0, ?, ?, 10, NULL, 'out', 'err', 'absent', NULL, 0, ?, ?, 11)`,
		ts, ts.Add(time.Second), ts, ts)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
INSERT INTO notifications (id, event_id, name, severity, subject_kind, subject_id, title, body, detail, read_at, occurred_at, created_at)
VALUES (31, 'evt-1', 'task.finished', 'info', 'task', '11', 'title', ?, ?, NULL, ?, ?)`,
		"long body text", `{"ok":true,"n":1}`, ts, ts)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
INSERT INTO audio_records (id, source_kind, source, source_url, original_filename, stored_path, mime_type, size_bytes, sha256, status, error_summary, played_at, created_at, updated_at)
VALUES (41, 'url', 'src', 'https://example.com/a.mp3', 'a.mp3', '/tmp/a.mp3', 'audio/mpeg', 123, 'abc', 'played', NULL, ?, ?, ?)`,
		ts, ts, ts)
	require.NoError(t, err)
}

// TestBusinessTablesCoverMigrateSchema 证明业务表目录覆盖全部 Ent migrate 表。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestBusinessTablesCoverMigrateSchema(t *testing.T) {
	got := map[string]struct{}{}
	for _, tb := range transfer.BusinessTables() {
		got[tb.Name] = struct{}{}
	}
	for _, tb := range migrate.Tables {
		if _, ok := got[tb.Name]; !ok {
			t.Fatalf("business catalog missing table %s", tb.Name)
		}
	}
	require.Equal(t, len(migrate.Tables), len(got))
}

// TestExportImportSQLiteRoundTrip 验证 SQLite 导出再导入保持计数与抽样内容。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestExportImportSQLiteRoundTrip(t *testing.T) {
	ctx := context.Background()
	src := openTempSQLite(t, "src.db")
	seedSampleData(t, src)

	out := filepath.Join(t.TempDir(), "dump.tdxfer")
	meta, err := transfer.Export(ctx, src, out, transfer.Options{})
	require.NoError(t, err)
	require.Equal(t, transfer.ExpectedFingerprint(), meta.SchemaFingerprint)
	require.Equal(t, 6, len(meta.Tables))

	dst := openTempSQLite(t, "dst.db")
	report, err := transfer.Import(ctx, dst, out, transfer.Options{})
	require.NoError(t, err)
	require.True(t, report.OK)

	v, err := transfer.VerifyStores(ctx, src, dst, transfer.Options{})
	require.NoError(t, err)
	require.True(t, v.OK, "verify: %+v", v)

	// 序列复位：插入新 admin 不应主键冲突
	_, err = dst.SQL().ExecContext(ctx, `
INSERT INTO admins (singleton_key, username, password_hash, active, created_at, updated_at)
VALUES ('singleton2', 'admin2', 'hash2', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
}

// TestTransferSQLiteToSQLite 验证 transfer 直连路径。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestTransferSQLiteToSQLite(t *testing.T) {
	ctx := context.Background()
	srcPath := filepath.Join(t.TempDir(), "src.db")
	dstPath := filepath.Join(t.TempDir(), "dst.db")

	src, err := data.Open(ctx, config.DatabaseConfig{Driver: "sqlite", DSN: "file:" + srcPath + "?_fk=1"})
	require.NoError(t, err)
	require.NoError(t, src.Migrate(ctx))
	defer src.Close()
	seedSampleData(t, src)

	report, err := transfer.Transfer(ctx,
		config.DatabaseConfig{Driver: "sqlite", DSN: "file:" + srcPath + "?_fk=1"},
		config.DatabaseConfig{Driver: "sqlite", DSN: "file:" + dstPath + "?_fk=1"},
		transfer.Options{},
	)
	require.NoError(t, err)
	require.True(t, report.OK)

	dst, err := data.Open(ctx, config.DatabaseConfig{Driver: "sqlite", DSN: "file:" + dstPath + "?_fk=1"})
	require.NoError(t, err)
	defer dst.Close()
	v, err := transfer.VerifyStores(ctx, src, dst, transfer.Options{})
	require.NoError(t, err)
	require.True(t, v.OK, "%+v", v)
}

// TestTargetNotEmptyRejected 验证目标非空默认拒绝。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestTargetNotEmptyRejected(t *testing.T) {
	ctx := context.Background()
	src := openTempSQLite(t, "src.db")
	seedSampleData(t, src)
	out := filepath.Join(t.TempDir(), "dump.tdxfer")
	_, err := transfer.Export(ctx, src, out, transfer.Options{})
	require.NoError(t, err)

	dst := openTempSQLite(t, "dst.db")
	seedSampleData(t, dst)
	_, err = transfer.Import(ctx, dst, out, transfer.Options{})
	require.Error(t, err)
	te, ok := transfer.AsError(err)
	require.True(t, ok)
	require.Equal(t, transfer.CodeTargetNotEmpty, te.Code)
}

// TestDryRunExportNoFile 验证 dry-run 不写文件。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestDryRunExportNoFile(t *testing.T) {
	ctx := context.Background()
	src := openTempSQLite(t, "src.db")
	seedSampleData(t, src)
	out := filepath.Join(t.TempDir(), "should-not-exist.tdxfer")
	meta, err := transfer.Export(ctx, src, out, transfer.Options{DryRun: true})
	require.NoError(t, err)
	require.NotNil(t, meta)
	require.NoFileExists(t, out)
}

// TestImportForceOverwrite 验证 --force 可覆盖非空目标。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestImportForceOverwrite(t *testing.T) {
	ctx := context.Background()
	src := openTempSQLite(t, "src.db")
	seedSampleData(t, src)
	out := filepath.Join(t.TempDir(), "dump.tdxfer")
	_, err := transfer.Export(ctx, src, out, transfer.Options{})
	require.NoError(t, err)

	dst := openTempSQLite(t, "dst.db")
	seedSampleData(t, dst)
	report, err := transfer.Import(ctx, dst, out, transfer.Options{Force: true})
	require.NoError(t, err)
	require.True(t, report.OK)
}

// TestBatchedImportWithCheckpoint 验证分批导入与 checkpoint completed。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestBatchedImportWithCheckpoint(t *testing.T) {
	ctx := context.Background()
	src := openTempSQLite(t, "src.db")
	seedSampleData(t, src)
	out := filepath.Join(t.TempDir(), "dump.tdxfer")
	_, err := transfer.Export(ctx, src, out, transfer.Options{})
	require.NoError(t, err)

	dst := openTempSQLite(t, "dst.db")
	cp := filepath.Join(t.TempDir(), "cp.json")
	report, err := transfer.Import(ctx, dst, out, transfer.Options{
		BatchSize:       1,
		CheckpointPath:  cp,
		SingleTxMaxRows: 1,
	})
	require.NoError(t, err)
	require.True(t, report.OK)
	require.FileExists(t, cp)
}

// TestSchemaMismatchOnBadFingerprint 验证 fingerprint 不一致时报 DBXFER_SCHEMA_MISMATCH。
//
// 参数:
//   - t: 测试上下文。
//
// 返回值:
//   - 无。
func TestSchemaMismatchOnBadFingerprint(t *testing.T) {
	ctx := context.Background()
	src := openTempSQLite(t, "src.db")
	seedSampleData(t, src)
	out := filepath.Join(t.TempDir(), "dump.tdxfer")
	_, err := transfer.Export(ctx, src, out, transfer.Options{})
	require.NoError(t, err)

	rawBytes, err := os.ReadFile(out)
	require.NoError(t, err)
	raw := strings.Replace(string(rawBytes), transfer.ExpectedFingerprint(), "deadbeef", 1)
	require.NoError(t, os.WriteFile(out, []byte(raw), 0o600))

	dst := openTempSQLite(t, "dst.db")
	_, err = transfer.Import(ctx, dst, out, transfer.Options{})
	require.Error(t, err)
	te, ok := transfer.AsError(err)
	require.True(t, ok)
	require.Equal(t, transfer.CodeSchemaMismatch, te.Code)
}
