package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestFlattenResponsesContentArraysForRetry_NoChange(t *testing.T) {
	raw := []byte(`{"model":"gpt-5.4","input":[{"type":"message","role":"user","content":"你好"}]}`)
	out, changed := flattenResponsesContentArraysForRetry(raw)
	if changed {
		t.Fatalf("expected unchanged, got changed=true body=%s", string(out))
	}
	if string(out) != string(raw) {
		t.Fatalf("expected identical body when unchanged")
	}
}

func TestFlattenResponsesContentArraysForRetry_FlattensMessageTextParts(t *testing.T) {
	raw := []byte(`{"input":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hello"},{"type":"output_text","text":"World"}]}]}`)
	out, changed := flattenResponsesContentArraysForRetry(raw)
	if !changed {
		t.Fatal("expected changed=true")
	}
	var req map[string]any
	if err := common.Unmarshal(out, &req); err != nil {
		t.Fatal(err)
	}
	input := req["input"].([]any)
	item := input[0].(map[string]any)
	content, ok := item["content"].(string)
	if !ok {
		t.Fatalf("expected flattened string content, got %#v", item["content"])
	}
	if content != "Hello\nWorld" {
		t.Fatalf("unexpected flattened content: %q", content)
	}
}

func TestFlattenResponsesContentArraysForRetry_StripsReasoningContentKeepsSummary(t *testing.T) {
	raw := []byte(`{"input":[{"type":"reasoning","content":[{"type":"reasoning_text","text":"internal"}],"encrypted_content":null}]}`)
	out, changed := flattenResponsesContentArraysForRetry(raw)
	if !changed {
		t.Fatal("expected changed=true")
	}
	var req map[string]any
	if err := common.Unmarshal(out, &req); err != nil {
		t.Fatal(err)
	}
	input := req["input"].([]any)
	item := input[0].(map[string]any)
	if _, exists := item["content"]; exists {
		t.Fatalf("expected reasoning.content removed, got %#v", item["content"])
	}
	summary, exists := item["summary"]
	if !exists {
		t.Fatal("expected reasoning.summary to exist")
	}
	summaryArr, ok := summary.([]any)
	if !ok || len(summaryArr) != 0 {
		t.Fatalf("expected empty summary array, got %#v", summary)
	}
}
