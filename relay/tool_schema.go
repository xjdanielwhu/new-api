package relay

import (
	"encoding/json"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert"
)

// normalizeChatToolSchemas rewrites function-parameters schemas produced by
// client-side schema generators (for example the Codex app "automation_update"
// MCP tool, which uses $defs/$ref plus a top-level oneOf). Strict upstream
// validators (OpenAI, Anthropic, Moonshot) reject those shapes; the rewrite
// inlines $defs/$ref and merges the top-level union into a plain object schema.
// Ordinary tool schemas are never touched. It returns the number of tools whose
// schema was rewritten.
func normalizeChatToolSchemas(request *dto.GeneralOpenAIRequest) int {
	normalized := 0
	for i := range request.Tools {
		params := request.Tools[i].Function.Parameters
		if relayconvert.IsComplexToolParameters(params) {
			request.Tools[i].Function.Parameters = relayconvert.NormalizeToolParameters(params)
			normalized++
		}
	}
	return normalized
}

// normalizeResponsesToolSchemas is the /v1/responses variant of
// normalizeChatToolSchemas. Tools travel as an opaque JSON array there, so the
// raw message is parsed, rewritten and re-marshaled only when at least one tool
// schema changed; otherwise the original bytes are returned.
func normalizeResponsesToolSchemas(tools json.RawMessage) (json.RawMessage, int) {
	if len(tools) == 0 || string(tools) == "null" {
		return tools, 0
	}
	var list []any
	if err := common.Unmarshal(tools, &list); err != nil {
		return tools, 0
	}
	normalized := 0
	for i, item := range list {
		tool, ok := item.(map[string]any)
		if !ok {
			continue
		}
		fn, ok := tool["function"].(map[string]any)
		if !ok {
			continue
		}
		params := fn["parameters"]
		if !relayconvert.IsComplexToolParameters(params) {
			continue
		}
		fn["parameters"] = relayconvert.NormalizeToolParameters(params)
		tool["function"] = fn
		list[i] = tool
		normalized++
	}
	if normalized == 0 {
		return tools, 0
	}
	out, err := common.Marshal(list)
	if err != nil {
		return tools, 0
	}
	return out, normalized
}
