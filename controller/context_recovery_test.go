package controller

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
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
