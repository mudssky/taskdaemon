package httpapi

// runLogMetaResponse 是日志元信息 API 响应。
type runLogMetaResponse struct {
	RunID       int     `json:"runId"`
	Status      string  `json:"status"`
	SizeBytes   *int64  `json:"sizeBytes"`
	CreatedAt   *string `json:"createdAt"`
	WriteFailed bool    `json:"writeFailed"`
	Available   bool    `json:"available"`
}

// runLogTailResponse 是尾部读取 API 响应。
type runLogTailResponse struct {
	RunID   int    `json:"runId"`
	Lines   int    `json:"lines"`
	Content string `json:"content"`
}

// runLogRangeResponse 是范围读取 API 响应。
type runLogRangeResponse struct {
	RunID   int    `json:"runId"`
	Offset  int64  `json:"offset"`
	Length  int64  `json:"length"`
	Content string `json:"content"`
}
