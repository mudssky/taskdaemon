package transfer

import (
	"encoding/json"
	"os"
	"time"
)

// loadCheckpoint 读取断点文件；路径为空或不存在时返回 nil。
//
// 参数:
//   - path: 断点路径。
//
// 返回值:
//   - *Checkpoint: 断点。
//   - error: 解析失败。
func loadCheckpoint(path string) (*Checkpoint, error) {
	if stringsTrim(path) == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, newError(CodeCheckpointInvalid, "read checkpoint", err)
	}
	var cp Checkpoint
	if err := json.Unmarshal(raw, &cp); err != nil {
		return nil, newError(CodeCheckpointInvalid, "parse checkpoint", err)
	}
	return &cp, nil
}

// saveCheckpoint 写入断点文件。
//
// 参数:
//   - path: 断点路径。
//   - cp: 断点内容。
//
// 返回值:
//   - error: 写入失败。
func saveCheckpoint(path string, cp Checkpoint) error {
	if stringsTrim(path) == "" {
		return nil
	}
	if cp.UpdatedAt.IsZero() {
		cp.UpdatedAt = time.Now().UTC()
	}
	raw, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return newError(CodeCheckpointInvalid, "encode checkpoint", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return newError(CodeCheckpointInvalid, "write checkpoint", err)
	}
	return nil
}

// stringsTrim 避免与 strings 循环导入式噪音（本地小包装）。
func stringsTrim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 {
		c := s[len(s)-1]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}
