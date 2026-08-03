package openai

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

// slowSSEReader 模拟上游 SSE 流：逐行延迟输出，给 ping goroutine 制造与数据写入交错的机会
type slowSSEReader struct {
	lines []string
	idx   int
	delay time.Duration
	buf   bytes.Buffer
}

func (r *slowSSEReader) Read(p []byte) (int, error) {
	for r.buf.Len() == 0 {
		if r.idx >= len(r.lines) {
			return 0, io.EOF
		}
		time.Sleep(r.delay)
		r.buf.WriteString(r.lines[r.idx] + "\n\n")
		r.idx++
	}
	return r.buf.Read(p)
}

func (r *slowSSEReader) Close() error { return nil }

func setupStreamTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	return c, w
}

// TestOaiResponsesStreamHandler_PingDataRace 验证 ping 与数据事件并发写入时
// SSE 流不会字节交错（每个事件完整独立），且 response.completed 正常结束
func TestOaiResponsesStreamHandler_PingDataRace(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 10
	defer func() { constant.StreamingTimeout = oldTimeout }()

	gs := operation_setting.GetGeneralSetting()
	oldEnabled, oldSeconds := gs.PingIntervalEnabled, gs.PingIntervalSeconds
	gs.PingIntervalEnabled = true
	gs.PingIntervalSeconds = 1
	defer func() { gs.PingIntervalEnabled, gs.PingIntervalSeconds = oldEnabled, oldSeconds }()

	c, w := setupStreamTestContext()
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-5.4"},
	}

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: &slowSSEReader{
			delay: 300 * time.Millisecond,
			lines: []string{
				`data: {"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`,
				`data: {"type":"response.output_text.delta","delta":"你"}`,
				`data: {"type":"response.output_text.delta","delta":"好"}`,
				`data: {"type":"response.output_text.delta","delta":"！"}`,
				`data: {"type":"response.completed","response":{"id":"resp_1","status":"completed","usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}}`,
			},
		},
	}

	usage, err := OaiResponsesStreamHandler(c, info, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usage.PromptTokens != 10 || usage.CompletionTokens != 5 {
		t.Fatalf("unexpected usage: %+v", usage)
	}

	body := w.Body.String()
	if !strings.Contains(body, "event: response.completed") {
		t.Fatalf("missing response.completed event in body:\n%s", body)
	}
	// 校验每一行都是合法 SSE 行（空行 / ping 注释 / event / data），
	// 任何 ping 与 data 字节交错都会产生非法行
	for i, line := range strings.Split(body, "\n") {
		if line == "" || line == ": PING" ||
			strings.HasPrefix(line, "event: ") || strings.HasPrefix(line, "data: ") {
			continue
		}
		t.Fatalf("corrupted SSE stream at line %d: %q\nfull body:\n%s", i, line, body)
	}
	// 数据事件应包含 ping 保活（响应约 1.5s，ping 间隔 1s）
	if !strings.Contains(body, ": PING") {
		t.Log("warning: no ping observed (timing dependent)")
	}
}

// TestOaiResponsesStreamHandler_EarlyEOF 上游未发 response.completed 即 EOF 时不应挂死
func TestOaiResponsesStreamHandler_EarlyEOF(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 10
	defer func() { constant.StreamingTimeout = oldTimeout }()

	c, _ := setupStreamTestContext()
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-5.4"},
	}

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: &slowSSEReader{
			delay: 50 * time.Millisecond,
			lines: []string{
				`data: {"type":"response.created","response":{"id":"resp_2","status":"in_progress"}}`,
				`data: {"type":"response.failed","response":{"id":"resp_2","status":"failed","error":{"code":"rate_limit_exceeded","message":"too many concurrent sessions"}}}`,
			},
		},
	}

	done := make(chan struct{})
	go func() {
		_, _ = OaiResponsesStreamHandler(c, info, resp)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(8 * time.Second):
		t.Fatal("stream handler hung after early EOF")
	}
}
