package template

// builtinDefinitions 返回首批内置备份模板。
//
// 参数:
//   - 无。
//
// 返回值:
//   - []Definition: 内置定义列表。
func builtinDefinitions() []Definition {
	return []Definition{
		postgresPgDumpDefinition(),
		sqliteFileBackupDefinition(),
		genericScriptDefinition(),
	}
}

// minMax 返回数值边界指针，供 ParamDef Min/Max 使用。
//
// 参数:
//   - v: 边界值。
//
// 返回值:
//   - *float64: 指向 v 的指针。
func minMax(v float64) *float64 {
	return new(v)
}
