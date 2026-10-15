package deepseek

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIResponsesRequestNormalizesWebSearchCallQueries(t *testing.T) {
	tests := []struct {
		name            string
		action          map[string]any
		topLevelQueries any
		wantQueries     []string
	}{
		{
			name:        "singular action query",
			action:      map[string]any{"type": "search", "query": "context compaction"},
			wantQueries: []string{"context compaction"},
		},
		{
			name:            "action queries win over stale top-level queries",
			action:          map[string]any{"type": "search", "queries": []any{"new query"}},
			topLevelQueries: []any{"stale query"},
			wantQueries:     []string{"new query"},
		},
		{
			name:            "top-level queries fill missing action queries",
			action:          map[string]any{"type": "search"},
			topLevelQueries: []any{"replayed query"},
			wantQueries:     []string{"replayed query"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			call := map[string]any{
				"type":   "web_search_call",
				"id":     "ws_1",
				"status": "completed",
				"action": tc.action,
			}
			if tc.topLevelQueries != nil {
				call["queries"] = tc.topLevelQueries
			}
			input, err := common.Marshal([]any{
				map[string]any{
					"type":    "message",
					"role":    "assistant",
					"content": "use search",
				},
				call,
			})
			require.NoError(t, err)

			converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, nil, dto.OpenAIResponsesRequest{
				Model: "deepseek-v4-flash-vision-exp",
				Input: input,
			})
			require.NoError(t, err)

			request, ok := converted.(dto.OpenAIResponsesRequest)
			require.True(t, ok)
			var gotInput []map[string]any
			require.NoError(t, common.Unmarshal(request.Input, &gotInput))
			require.Len(t, gotInput, 2)

			searchCall := gotInput[1]
			wantQueries := make([]any, len(tc.wantQueries))
			for i, query := range tc.wantQueries {
				wantQueries[i] = query
			}
			require.Equal(t, wantQueries, searchCall["queries"])
			require.NotContains(t, searchCall, "query")

			action, ok := searchCall["action"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "search", action["type"])
			require.Equal(t, wantQueries, action["queries"])
			require.NotContains(t, action, "query")
		})
	}
}
