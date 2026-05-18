package httpapi

import (
	"bytes"
	"io"

	"github.com/gin-gonic/gin"
)

type loggedBody struct {
	content   []byte
	truncated bool
}

type loggedResponseWriter struct {
	gin.ResponseWriter
	limit     int
	body      bytes.Buffer
	truncated bool
}

// newLoggedResponseWriter 创建会透传响应并缓存有限 body 的 writer。
//
// 参数:
//   - writer: Gin 原始响应 writer。
//   - limit: 最大缓存字节数。
//
// 返回值:
//   - *loggedResponseWriter: 响应体缓存 writer。
func newLoggedResponseWriter(writer gin.ResponseWriter, limit int) *loggedResponseWriter {
	return &loggedResponseWriter{
		ResponseWriter: writer,
		limit:          limit,
	}
}

// Write 写出响应并缓存有限字节用于日志。
//
// 参数:
//   - data: 响应片段。
//
// 返回值:
//   - int: 实际写出的字节数。
//   - error: 底层 writer 返回的错误。
func (writer *loggedResponseWriter) Write(data []byte) (int, error) {
	writer.capture(data)
	return writer.ResponseWriter.Write(data)
}

// WriteString 写出字符串响应并缓存有限字节用于日志。
//
// 参数:
//   - data: 响应字符串片段。
//
// 返回值:
//   - int: 实际写出的字节数。
//   - error: 底层 writer 返回的错误。
func (writer *loggedResponseWriter) WriteString(data string) (int, error) {
	writer.capture([]byte(data))
	return writer.ResponseWriter.WriteString(data)
}

// capture 缓存不超过限制的响应体片段。
//
// 参数:
//   - data: 响应片段。
//
// 返回值:
//   - 无。
func (writer *loggedResponseWriter) capture(data []byte) {
	if writer.limit <= 0 || len(data) == 0 {
		if len(data) > 0 {
			writer.truncated = true
		}
		return
	}
	remaining := writer.limit - writer.body.Len()
	if remaining <= 0 {
		writer.truncated = true
		return
	}
	if len(data) > remaining {
		writer.body.Write(data[:remaining])
		writer.truncated = true
		return
	}
	writer.body.Write(data)
}

// loggedBody 返回缓存到的响应体。
//
// 参数:
//   - 无。
//
// 返回值:
//   - loggedBody: 响应体片段与截断状态。
func (writer *loggedResponseWriter) loggedBody() loggedBody {
	return loggedBody{
		content:   writer.body.Bytes(),
		truncated: writer.truncated,
	}
}

// captureRequestBody 读取并恢复请求体，供日志和后续 handler 共同使用。
//
// 参数:
//   - ctx: Gin 请求上下文。
//   - limit: 最大记录字节数。
//
// 返回值:
//   - loggedBody: 请求体片段与截断状态。
func captureRequestBody(ctx *gin.Context, limit int) loggedBody {
	if ctx.Request == nil || ctx.Request.Body == nil {
		return loggedBody{}
	}
	body, err := io.ReadAll(io.LimitReader(ctx.Request.Body, int64(limit)+1))
	if err != nil {
		ctx.Error(err)
		ctx.Request.Body = newCompositeReadCloser(bytes.NewReader(body), ctx.Request.Body)
		return limitedLoggedBody(body, limit)
	}
	ctx.Request.Body = newCompositeReadCloser(bytes.NewReader(body), ctx.Request.Body)
	return limitedLoggedBody(body, limit)
}

type compositeReadCloser struct {
	reader io.Reader
	closer io.Closer
}

// newCompositeReadCloser 将预读 body 与剩余原始流重新组合。
//
// 参数:
//   - prefix: 已经为了日志预读的 body 片段。
//   - rest: 原始请求体剩余流。
//
// 返回值:
//   - io.ReadCloser: 供后续 handler 正常读取的完整请求体。
func newCompositeReadCloser(prefix io.Reader, rest io.ReadCloser) io.ReadCloser {
	return compositeReadCloser{
		reader: io.MultiReader(prefix, rest),
		closer: rest,
	}
}

// Read 读取组合后的请求体。
//
// 参数:
//   - data: 读取缓冲区。
//
// 返回值:
//   - int: 读取字节数。
//   - error: 读取失败或 EOF 时返回的错误。
func (reader compositeReadCloser) Read(data []byte) (int, error) {
	return reader.reader.Read(data)
}

// Close 关闭底层原始请求体。
//
// 参数:
//   - 无。
//
// 返回值:
//   - error: 底层 Close 返回的错误。
func (reader compositeReadCloser) Close() error {
	return reader.closer.Close()
}

// limitedLoggedBody 返回限制长度后的 body。
//
// 参数:
//   - body: 原始 body。
//   - limit: 最大记录字节数。
//
// 返回值:
//   - loggedBody: body 片段与截断状态。
func limitedLoggedBody(body []byte, limit int) loggedBody {
	if limit <= 0 {
		return loggedBody{truncated: len(body) > 0}
	}
	if len(body) > limit {
		return loggedBody{
			content:   body[:limit],
			truncated: true,
		}
	}
	return loggedBody{content: body}
}
