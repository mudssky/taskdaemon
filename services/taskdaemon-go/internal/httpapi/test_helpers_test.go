package httpapi

import (
	"encoding/json"
	"testing"
)

// decodeAPIData 从响应 envelope 中解出 data。
//
// 参数:
//   - t: Go 测试上下文。
//   - body: HTTP 响应体。
//   - target: data 解码目标。
//
// 返回值:
//   - apiResponse: 原始响应 envelope。
func decodeAPIData(t *testing.T, body []byte, target any) apiResponse {
	t.Helper()
	var envelope apiResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Code != responseEnvelopeSuccessCode {
		t.Fatalf("envelope code = %d, want success", envelope.Code)
	}
	data, err := json.Marshal(envelope.Data)
	if err != nil {
		t.Fatalf("marshal envelope data: %v", err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("decode envelope data: %v", err)
	}
	return envelope
}

// decodeAPIError 从响应 envelope 中解出错误对象。
//
// 参数:
//   - t: Go 测试上下文。
//   - body: HTTP 响应体。
//
// 返回值:
//   - apiResponse: 原始错误响应 envelope。
func decodeAPIError(t *testing.T, body []byte) apiResponse {
	t.Helper()
	var envelope apiResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Code != responseEnvelopeErrorCode {
		t.Fatalf("envelope code = %d, want error", envelope.Code)
	}
	if envelope.Data != nil {
		t.Fatalf("envelope data = %v, want nil", envelope.Data)
	}
	if envelope.Error == nil {
		t.Fatal("envelope error is nil")
	}
	return envelope
}
