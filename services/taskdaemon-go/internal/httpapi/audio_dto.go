package httpapi

import (
	"time"

	"taskdaemon/internal/audio"
	"taskdaemon/internal/data/ent"
)

type submitAudioURLRequest struct {
	URL      string `json:"url"`
	Source   string `json:"source"`
	Filename string `json:"filename"`
	MIMEType string `json:"mimeType"`
}

type audioRecordResponse struct {
	ID               int        `json:"id"`
	SourceKind       string     `json:"sourceKind"`
	Source           string     `json:"source"`
	OriginalFilename string     `json:"originalFilename"`
	StoredPath       string     `json:"storedPath"`
	MIMEType         string     `json:"mimeType"`
	SizeBytes        int64      `json:"sizeBytes"`
	SHA256           string     `json:"sha256"`
	Status           string     `json:"status"`
	ErrorSummary     string     `json:"errorSummary"`
	PlayedAt         *time.Time `json:"playedAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

// audioRecordResponseFromEnt 将 Ent 音频记录转换为 API 响应 DTO。
//
// 参数:
//   - record: Ent 音频记录。
//
// 返回值:
//   - audioRecordResponse: API 响应 DTO。
func audioRecordResponseFromEnt(record *ent.AudioRecord) audioRecordResponse {
	if record == nil {
		return audioRecordResponse{}
	}
	return audioRecordResponse{
		ID:               record.ID,
		SourceKind:       string(record.SourceKind),
		Source:           record.Source,
		OriginalFilename: record.OriginalFilename,
		StoredPath:       record.StoredPath,
		MIMEType:         record.MimeType,
		SizeBytes:        record.SizeBytes,
		SHA256:           record.Sha256,
		Status:           string(record.Status),
		ErrorSummary:     record.ErrorSummary,
		PlayedAt:         record.PlayedAt,
		CreatedAt:        record.CreatedAt,
		UpdatedAt:        record.UpdatedAt,
	}
}

// audioURLInputFromRequest 将 JSON 请求 DTO 转为音频服务输入。
//
// 参数:
//   - req: URL 播放请求 DTO。
//
// 返回值:
//   - audio.URLRequest: 音频服务输入。
func audioURLInputFromRequest(req submitAudioURLRequest) audio.URLRequest {
	return audio.URLRequest{
		URL:      req.URL,
		Source:   req.Source,
		Filename: req.Filename,
		MIMEType: req.MIMEType,
	}
}
