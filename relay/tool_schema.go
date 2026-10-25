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
		if rewritten, ok := relayconvert.NormalizeToolParametersJSON(params); ok {
			request.Tools[i].Function.Parameters = rewritten
			normalized++
		}
	}
	return normalized
}

// normalizeResponsesToolSchemas is the /v1/responses variant of
// normalizeChatToolSchemas. Tools travel as an opaque JSON array there, so the
// raw message is parsed, rewritten and re-marshaled only when at least one tool
// schema changed; otherwise the original bytes are returned. Unlike Chat
// Completions, a Responses function tool carries "parameters" directly on the
// tool object instead of a nested "function"; a namespace tool additionally
// nests its member tools under "tools", so each member is checked too.
func normalizeResponsesToolSchemas(tools json.RawMessage) (json.RawMessage, int) {
	if len(tools) == 0 || string(tools) == "null" {
		return tools, 0
	}
	var list []any
	if err := common.Unmarshal(tools, &list); err != nil {
		return tools, 0
	}
	normalized := 0
	for _, item := range list {
		tool, ok := item.(map[string]any)
		if !ok {
			continue
		}
		normalized += normalizeResponsesToolParameters(tool)
		if members, ok := tool["tools"].([]any); ok {
			for _, member := range members {
				if memberTool, ok := member.(map[string]any); ok {
					normalized += normalizeResponsesToolParameters(memberTool)
				}
			}
		}
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

// normalizeResponsesToolParameters rewrites the "parameters" field in place on
// a single Responses tool object (a top-level function tool or a namespace
// member) when it uses a generated schema shape. It returns 1 when rewritten,
// 0 otherwise.
func normalizeResponsesToolParameters(tool map[string]any) int {
	params, exists := tool["parameters"]
	if !exists || !relayconvert.IsComplexToolParameters(params) {
		return 0
	}
	tool["parameters"] = relayconvert.NormalizeToolParameters(params)
	return 1
}
