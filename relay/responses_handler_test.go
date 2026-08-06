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

func TestSummarizeResponsesReasoningItemsForRetry_ConvertsSummaryToAssistantMessage(t *testing.T) {
	raw := []byte(`{"input":[{"type":"reasoning","summary":[{"type":"summary_text","text":"first"},{"type":"summary_text","text":"second"}]}]}`)
	out, changed := summarizeResponsesReasoningItemsForRetry(raw)
	if !changed {
		t.Fatal("expected changed=true")
	}
	var req map[string]any
	if err := common.Unmarshal(out, &req); err != nil {
		t.Fatal(err)
	}
	input := req["input"].([]any)
	if len(input) != 1 {
		t.Fatalf("expected one converted input item, got %d", len(input))
	}
	item := input[0].(map[string]any)
	if item["type"] != "message" {
		t.Fatalf("expected converted item type=message, got %#v", item["type"])
	}
	if item["role"] != "assistant" {
		t.Fatalf("expected converted role=assistant, got %#v", item["role"])
	}
	content, _ := item["content"].(string)
	if content != "[Context Summary]\nfirst\n\nsecond" {
		t.Fatalf("unexpected summarized content: %q", content)
	}
}

func TestStripResponsesReasoningItemsForRetry_RemovesReasoningItems(t *testing.T) {
	raw := []byte(`{"input":[{"type":"reasoning","summary":[{"type":"summary_text","text":"first"}]},{"type":"message","role":"user","content":"hello"}]}`)
	out, changed := stripResponsesReasoningItemsForRetry(raw)
	if !changed {
		t.Fatal("expected changed=true")
	}
	var req map[string]any
	if err := common.Unmarshal(out, &req); err != nil {
		t.Fatal(err)
	}
	input := req["input"].([]any)
	if len(input) != 1 {
		t.Fatalf("expected one remaining input item, got %d", len(input))
	}
	item := input[0].(map[string]any)
	if item["type"] != "message" {
		t.Fatalf("expected remaining item type=message, got %#v", item["type"])
	}
}
