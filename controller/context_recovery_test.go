package controller

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestStripOldImagePartsChatMessages(t *testing.T) {
	img := func() map[string]any {
		return map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,AAAA"}}
	}
	messages := []any{
		map[string]any{"role": "user", "content": []any{img(), map[string]any{"type": "text", "text": "old"}}},
		map[string]any{"role": "assistant", "content": "ok"},
		map[string]any{"role": "user", "content": []any{img(), map[string]any{"type": "text", "text": "new"}}},
	}
	changed := stripOldImageParts(messages, []string{"content"}, "text")
	if !changed {
		t.Fatal("expected changed=true")
	}
	// 第一条消息的图片应被替换为文本占位
	first := messages[0].(map[string]any)["content"].([]any)
	if first[0].(map[string]any)["type"] != "text" {
		t.Fatalf("expected old image replaced with text part, got %v", first[0])
	}
	if first[0].(map[string]any)["text"] != omittedImagePlaceholder {
		t.Fatalf("unexpected placeholder: %v", first[0])
	}
	// 最后一条含图消息的图片必须保留
	last := messages[2].(map[string]any)["content"].([]any)
	if last[0].(map[string]any)["type"] != "image_url" {
		t.Fatalf("expected last image kept, got %v", last[0])
	}
}

func TestStripOldImagePartsResponsesInput(t *testing.T) {
	imgPart := func() map[string]any {
		return map[string]any{"type": "input_image", "image_url": "data:image/png;base64,BBBB"}
	}
	input := []any{
		map[string]any{"type": "message", "role": "user", "content": []any{imgPart()}},
		map[string]any{"type": "function_call_output", "call_id": "fc_1", "output": []any{imgPart()}},
		map[string]any{"type": "reasoning", "id": "rs_1"},
		map[string]any{"type": "message", "role": "user", "content": []any{imgPart()}},
	}
	changed := stripOldImageParts(input, []string{"content", "output"}, "input_text")
	if !changed {
		t.Fatal("expected changed=true")
	}
	// message content 中的旧图片应替换为 input_text 占位
	first := input[0].(map[string]any)["content"].([]any)
	if first[0].(map[string]any)["type"] != "input_text" {
		t.Fatalf("expected input_text placeholder, got %v", first[0])
	}
	// function_call_output 的 output 数组中的图片也应剥离
	fco := input[1].(map[string]any)["output"].([]any)
	if fco[0].(map[string]any)["type"] != "input_text" {
		t.Fatalf("expected function_call_output image stripped, got %v", fco[0])
	}
	// reasoning item 不含图片，不受影响
	if input[2].(map[string]any)["id"] != "rs_1" {
		t.Fatalf("reasoning item should be untouched: %v", input[2])
	}
	// 最后一条含图 item 保留
	last := input[3].(map[string]any)["content"].([]any)
	if last[0].(map[string]any)["type"] != "input_image" {
		t.Fatalf("expected last image kept, got %v", last[0])
	}
}

func TestStripOldImagePartsNoImage(t *testing.T) {
	messages := []any{
		map[string]any{"role": "user", "content": "hello"},
		map[string]any{"role": "assistant", "content": "hi"},
	}
	if stripOldImageParts(messages, []string{"content"}, "text") {
		t.Fatal("expected changed=false for text-only messages")
	}
}

// 模拟 413 恢复路径：请求体含 messages + 大量 base64 图片，剥离后请求体应显著缩小
func TestPayloadRecoveryShrinksBody(t *testing.T) {
	bigImage := "data:image/png;base64," + strings.Repeat("A", 500000)
	raw := []byte(`{"model":"gpt-5.4","input":[` +
		`{"type":"message","role":"user","content":[{"type":"input_image","image_url":"` + bigImage + `"},{"type":"input_text","text":"img 1"}]},` +
		`{"type":"message","role":"user","content":[{"type":"input_image","image_url":"` + bigImage + `"},{"type":"input_text","text":"img 2"}]},` +
		`{"type":"message","role":"user","content":[{"type":"input_image","image_url":"` + bigImage + `"},{"type":"input_text","text":"img 3"}]}` +
		`]}`)

	var reqMap map[string]any
	if err := common.Unmarshal(raw, &reqMap); err != nil {
		t.Fatal(err)
	}
	input := reqMap["input"].([]any)
	if !stripOldImageParts(input, []string{"content", "output"}, "input_text") {
		t.Fatal("expected changed=true")
	}
	reqMap["input"] = input
	newBody, err := common.Marshal(reqMap)
	if err != nil {
		t.Fatal(err)
	}
	if len(newBody) >= len(raw)/2 {
		t.Fatalf("expected body shrunk significantly, before=%d after=%d", len(raw), len(newBody))
	}
	if !strings.Contains(string(newBody), omittedImagePlaceholder) {
		t.Fatal("expected placeholder in new body")
	}
	// 最新图片应保留完整 data url
	if !strings.Contains(string(newBody), bigImage) {
		t.Fatal("expected latest image data url kept")
	}
}

func TestParseContextOverflowMaxTokens(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    int
		ok      bool
	}{
		{
			name:    "openai",
			message: "This model's maximum context length is 128000 tokens. However, your messages resulted in 130000 tokens. Please reduce the length of the messages or completion.",
			want:    128000,
			ok:      true,
		},
		{
			name:    "deepseek",
			message: "This model's maximum context length is 1048565 tokens. However, you requested 3220751 tokens",
			want:    1048565,
			ok:      true,
		},
		{
			name:    "oc configured limit",
			message: "Input tokens exceed the configured limit of 922000 tokens. Your messages resulted in 1055751 tokens. Please reduce the length of the messages.",
			want:    922000,
			ok:      true,
		},
		{
			name:    "kimi model token limit",
			message: "Invalid request: Your request exceeded model token limit: 262144 (requested: 1055340)",
			want:    262144,
			ok:      true,
		},
		{
			name:    "not a context overflow",
			message: "insufficient quota",
			ok:      false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseContextOverflowMaxTokens(tc.message)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("parseContextOverflowMaxTokens(%q) = (%d, %v), want (%d, %v)", tc.message, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestIsImageUnsupportedError(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    bool
	}{
		{
			name:    "does not support images",
			message: "Invalid request: This model does not support images in the conversation.",
			want:    true,
		},
		{
			name:    "unknown variant image_url (deepseek-v4-flash)",
			message: "Failed to deserialize the JSON body into the target type: messages[7]: unknown variant `image_url`, expected `text` at line 1 column 1626970",
			want:    true,
		},
		{
			name:    "unknown variant image (double-quoted)",
			message: `Failed to deserialize: unknown variant "image_url", expected text`,
			want:    true,
		},
		{
			name:    "image input not supported",
			message: "Image input is not supported for this model.",
			want:    true,
		},
		{
			name:    "unsupported image",
			message: "Unsupported image content type.",
			want:    true,
		},
		{
			name:    "no vision support",
			message: "The model does not support vision inputs.",
			want:    true,
		},
		{
			name:    "unrelated error",
			message: "Insufficient quota for the request.",
			want:    false,
		},
		{
			name:    "context overflow",
			message: "Your request exceeded model token limit: 262144 (requested: 1055340)",
			want:    false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isImageUnsupportedError(tc.message); got != tc.want {
				t.Fatalf("isImageUnsupportedError(%q) = %v, want %v", tc.message, got, tc.want)
			}
		})
	}
}

func TestStripAllImagePartsChatMessages(t *testing.T) {
	img := func() map[string]any {
		return map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,AAAA"}}
	}
	messages := []any{
		map[string]any{"role": "user", "content": []any{img(), map[string]any{"type": "text", "text": "看图"}}},
		map[string]any{"role": "assistant", "content": "ok"},
		map[string]any{"role": "user", "content": []any{img(), img()}},
	}
	removed, changed := stripAllImageParts(messages, []string{"content"}, "text")
	if !changed || removed != 3 {
		t.Fatalf("expected changed=true removed=3, got changed=%v removed=%d", changed, removed)
	}
	placeholderSeen := false
	for i, m := range messages {
		mm := m.(map[string]any)
		content, ok := mm["content"].([]any)
		if !ok {
			continue
		}
		for _, p := range content {
			pm := p.(map[string]any)
			if pm["type"] == "text" && pm["text"] == imageRemovedPlaceholder {
				placeholderSeen = true
				continue
			}
			if pm["type"] == "image_url" {
				t.Fatalf("message %d: image part should have been removed, got %v", i, p)
			}
		}
	}
	if !placeholderSeen {
		t.Fatal("expected at least one imageRemovedPlaceholder in messages")
	}
}

func TestStripAllImagePartsResponsesInput(t *testing.T) {
	imgPart := func() map[string]any {
		return map[string]any{"type": "input_image", "image_url": "data:image/png;base64,BBBB"}
	}
	input := []any{
		map[string]any{"type": "message", "role": "user", "content": []any{imgPart()}},
		map[string]any{"type": "function_call_output", "call_id": "fc_1", "output": []any{imgPart()}},
	}
	removed, changed := stripAllImageParts(input, []string{"content", "output"}, "input_text")
	if !changed || removed != 2 {
		t.Fatalf("expected changed=true removed=2, got changed=%v removed=%d", changed, removed)
	}
	first := input[0].(map[string]any)["content"].([]any)[0].(map[string]any)
	if first["type"] != "input_text" || first["text"] != imageRemovedPlaceholder {
		t.Fatalf("expected input_text placeholder, got %v", first)
	}
	fco := input[1].(map[string]any)["output"].([]any)[0].(map[string]any)
	if fco["type"] != "input_text" || fco["text"] != imageRemovedPlaceholder {
		t.Fatalf("expected function_call_output placeholder, got %v", fco)
	}
}

func TestIsToolPairingErrorNoToolOutputFound(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    bool
	}{
		{
			name:    "cooai no tool output found",
			message: "No tool output found for tool call call_02_9r7hnOiZfy9N67N7azD82137.",
			want:    true,
		},
		{
			name:    "no output found for function call",
			message: "No output found for function call call_abc123",
			want:    true,
		},
		{
			name:    "must be followed by tool messages",
			message: "assistant message with 'tool_calls' must be followed by tool messages",
			want:    true,
		},
		{
			name:    "unrelated error",
			message: "The server had an error while processing your request.",
			want:    false,
		},
	}
	for _, tc := range cases {
		if got := isToolPairingError(tc.message); got != tc.want {
			t.Fatalf("%s: isToolPairingError(%q) = %v, want %v", tc.name, tc.message, got, tc.want)
		}
	}
}

func TestCompressImagesInBodyChatMessages(t *testing.T) {
	imgPart := func() map[string]any {
		// 大尺寸 base64 PNG，压缩后必然变小
		dataURL := testCompressPNGDataURL(t, 1600, 1200)
		return map[string]any{"type": "image_url", "image_url": map[string]any{"url": dataURL}}
	}
	reqMap := map[string]any{
		"model": "chatgpt-5.4",
		"messages": []any{
			map[string]any{"role": "user", "content": []any{imgPart(), map[string]any{"type": "text", "text": "用这张参考图做海报"}}},
		},
	}
	raw, err := common.Marshal(reqMap)
	if err != nil {
		t.Fatal(err)
	}
	if !compressImagesInBody(reqMap) {
		t.Fatal("expected compressImagesInBody changed=true")
	}
	newBody, err := common.Marshal(reqMap)
	if err != nil {
		t.Fatal(err)
	}
	if len(newBody) >= len(raw) {
		t.Fatalf("expected body shrunk: before=%d after=%d", len(raw), len(newBody))
	}
	var parsed map[string]any
	if err := common.Unmarshal(newBody, &parsed); err != nil {
		t.Fatal(err)
	}
	content := parsed["messages"].([]any)[0].(map[string]any)["content"].([]any)
	url := content[0].(map[string]any)["image_url"].(map[string]any)["url"].(string)
	if !strings.HasPrefix(url, "data:image/jpeg;base64,") {
		t.Fatalf("expected compressed jpeg data url, got prefix: %s", url[:40])
	}
}

func TestCompressImagesInBodyResponsesInput(t *testing.T) {
	dataURL := testCompressPNGDataURL(t, 1600, 1200)
	reqMap := map[string]any{
		"model": "chatgpt-5.4",
		"input": []any{
			map[string]any{"type": "message", "role": "user", "content": []any{
				map[string]any{"type": "input_image", "image_url": dataURL},
			}},
		},
	}
	raw, err := common.Marshal(reqMap)
	if err != nil {
		t.Fatal(err)
	}
	if !compressImagesInBody(reqMap) {
		t.Fatal("expected compressImagesInBody changed=true")
	}
	newBody, err := common.Marshal(reqMap)
	if err != nil {
		t.Fatal(err)
	}
	if len(newBody) >= len(raw) {
		t.Fatalf("expected body shrunk: before=%d after=%d", len(raw), len(newBody))
	}
	var parsed map[string]any
	if err := common.Unmarshal(newBody, &parsed); err != nil {
		t.Fatal(err)
	}
	content := parsed["input"].([]any)[0].(map[string]any)["content"].([]any)
	url := content[0].(map[string]any)["image_url"].(string)
	if !strings.HasPrefix(url, "data:image/jpeg;base64,") {
		t.Fatalf("expected compressed jpeg data url, got prefix: %s", url[:40])
	}
}

func TestCompressImagesInBodyNoChangeWhenSmallOrAbsent(t *testing.T) {
	reqMap := map[string]any{
		"model": "chatgpt-5.4",
		"input": []any{
			map[string]any{"type": "message", "role": "user", "content": "没有图片"},
		},
	}
	if compressImagesInBody(reqMap) {
		t.Fatal("expected no change for text-only body")
	}
}

func testCompressPNGDataURL(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	rnd := rand.New(rand.NewSource(42))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(rnd.Intn(256)), G: uint8(rnd.Intn(256)), B: uint8(rnd.Intn(256)), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestIsReasoningEffortWithToolsError(t *testing.T) {
	cases := []struct {
		name string
		msg  string
		want bool
	}{
		{
			name: "gpt-5.4 chat rejection",
			msg:  "Function tools with reasoning_effort are not supported for gpt-5.4 in /v1/chat/completions. To use function tools, use /v1/responses or set reasoning_effort to 'none'.",
			want: true,
		},
		{
			name: "lowercase variant",
			msg:  "function tools with reasoning_effort are not supported",
			want: true,
		},
		{
			name: "unrelated error",
			msg:  "The model does not support images",
			want: false,
		},
		{
			name: "tool pairing error",
			msg:  "No tool output found for tool call call_02_9r7hnOiZfy9N67N7azD82137",
			want: false,
		},
	}
	for _, tc := range cases {
		if got := isReasoningEffortWithToolsError(tc.msg); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestReasoningEffortDowngradeMutatesBody(t *testing.T) {
	raw := []byte(`{"model":"gpt-5.4","reasoning_effort":"high","tools":[{"type":"function","function":{"name":"get_weather"}}],"messages":[{"role":"user","content":"hi"}]}`)
	var reqMap map[string]any
	if err := common.Unmarshal(raw, &reqMap); err != nil {
		t.Fatal(err)
	}
	effort, ok := reqMap["reasoning_effort"].(string)
	if !ok || !strings.EqualFold(effort, "high") {
		t.Fatalf("expected reasoning_effort=high, got %v", reqMap["reasoning_effort"])
	}
	if _, hasTools := reqMap["tools"]; !hasTools {
		t.Fatal("expected tools present")
	}
	reqMap["reasoning_effort"] = "none"
	if effort, _ := reqMap["reasoning_effort"].(string); !strings.EqualFold(effort, "none") {
		t.Fatalf("expected reasoning_effort downgraded to none, got %v", reqMap["reasoning_effort"])
	}
	if _, hasTools := reqMap["tools"]; !hasTools {
		t.Fatal("expected tools preserved after downgrade")
	}
}

// 413 恢复必须覆盖 responses 系列：/v1/responses 使用 RelayFormatOpenAIResponses，
// 若只判定 relayFormat == RelayFormatOpenAI，Codex 的 responses 会话在累积大量
// base64 图片被上游 413 拒绝后将永远无法恢复（每次重试都重发同一个超大请求体）。
func TestSupportsPayloadImageRecovery(t *testing.T) {
	cases := []struct {
		name        string
		relayFormat types.RelayFormat
		relayMode   int
		want        bool
	}{
		{"chat completions", types.RelayFormatOpenAI, relayconstant.RelayModeChatCompletions, true},
		{"responses", types.RelayFormatOpenAIResponses, relayconstant.RelayModeResponses, true},
		{"responses compact", types.RelayFormatOpenAIResponsesCompaction, relayconstant.RelayModeResponsesCompact, true},
		{"openai completions", types.RelayFormatOpenAI, relayconstant.RelayModeCompletions, false},
		{"openai embeddings", types.RelayFormatOpenAI, relayconstant.RelayModeEmbeddings, false},
		{"claude", types.RelayFormatClaude, relayconstant.RelayModeChatCompletions, false},
		{"gemini", types.RelayFormatGemini, relayconstant.RelayModeChatCompletions, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, supportsPayloadImageRecovery(tc.relayFormat, tc.relayMode))
		})
	}
}

// 自动恢复开关默认由环境变量控制（生产默认 true），单测需显式初始化。
func enableAutoRecoveryForTest(t *testing.T) {
	t.Helper()
	prev := constant.ContextOverflowAutoRecovery
	constant.ContextOverflowAutoRecovery = true
	t.Cleanup(func() { constant.ContextOverflowAutoRecovery = prev })
}

func newResponsesRecoveryContext(t *testing.T, raw []byte) *gin.Context {
	t.Helper()
	enableAutoRecoveryForTest(t)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(raw))
	storage, err := common.CreateBodyStorage(raw)
	require.NoError(t, err)
	c.Set(common.KeyBodyStorage, storage)
	return c
}

func payloadTooLargeError() *types.NewAPIError {
	return types.NewErrorWithStatusCode(errors.New("bad response status code 413"), types.ErrorCodeBadResponseStatusCode, http.StatusRequestEntityTooLarge)
}

// 模拟真实 Codex responses 请求：多张 function_call_output 中的 base64 图片
// （type=input_image，位于 output 数组内）使请求体超过上游上限返回 413。
// 恢复后旧的图片必须被剥离、最新一张保留，且 info.Request 同步更新。
func TestTryPayloadTooLargeRecoveryForResponsesInput(t *testing.T) {
	oldImage := "data:image/png;base64," + strings.Repeat("A", 400000)
	newImage := "data:image/png;base64," + strings.Repeat("B", 400000)
	raw := []byte(`{"model":"deepseek-v4-flash-vision-exp","input":[` +
		`{"type":"message","role":"user","content":[{"type":"input_image","image_url":"` + oldImage + `"},{"type":"input_text","text":"old"}]},` +
		`{"type":"function_call_output","call_id":"call_1","output":[{"type":"input_image","image_url":"` + oldImage + `","detail":"high"}]},` +
		`{"type":"function_call_output","call_id":"call_2","output":[{"type":"input_image","image_url":"` + newImage + `","detail":"high"}]}` +
		`]}`)

	c := newResponsesRecoveryContext(t, raw)
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		Request:   &dto.OpenAIResponsesRequest{},
	}

	require.True(t, tryPayloadTooLargeRecovery(c, info, payloadTooLargeError()))

	newStorage, err := common.GetBodyStorage(c)
	require.NoError(t, err)
	newBody, err := newStorage.Bytes()
	require.NoError(t, err)
	require.Less(t, len(newBody), len(raw))
	require.Contains(t, string(newBody), omittedImagePlaceholder)
	require.NotContains(t, string(newBody), oldImage)
	require.Contains(t, string(newBody), newImage, "最新一张图片必须保留")

	updated, err := common.Marshal(info.Request)
	require.NoError(t, err)
	require.Contains(t, string(updated), omittedImagePlaceholder)
	require.NotContains(t, string(updated), oldImage)
}

func TestTryPayloadTooLargeRecoverySkipsNon413AndTextOnlyResponses(t *testing.T) {
	raw := []byte(`{"model":"deepseek-v4-flash-vision-exp","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}]}`)

	c := newResponsesRecoveryContext(t, raw)
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		Request:   &dto.OpenAIResponsesRequest{},
	}
	notFound := types.NewErrorWithStatusCode(errors.New("bad response status code 404"), types.ErrorCodeBadResponseStatusCode, http.StatusNotFound)
	require.False(t, tryPayloadTooLargeRecovery(c, info, notFound), "非 413 不应触发恢复")

	require.False(t, tryPayloadTooLargeRecovery(c, info, payloadTooLargeError()), "无可剥离图片时不应触发恢复")
}
