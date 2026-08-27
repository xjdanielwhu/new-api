package zhipu_4v

import "testing"

func TestNormalizeGlm53ReasoningEffort(t *testing.T) {
	cases := []struct {
		name   string
		model  string
		effort string
		want   string
	}{
		{"none downgraded", "glm-5.3", "none", "low"},
		{"disabled downgraded", "glm-5.3", "disabled", "low"},
		{"empty downgraded", "glm-5.3", "", "low"},
		{"low preserved", "glm-5.3", "low", "low"},
		{"high preserved", "glm-5.3", "high", "high"},
		{"max preserved", "glm-5.3", "max", "max"},
		{"uppercase preserved", "glm-5.3", "HIGH", "HIGH"},
		{"invalid downgraded", "glm-5.3", "minimal", "low"},
		{"other model untouched", "glm-5.2", "none", "none"},
		{"other model low untouched", "gpt-5.4", "none", "none"},
		{"glm-5v-turbo untouched", "glm-5v-turbo", "none", "none"},
		{"glm-5.32 untouched", "glm-5.32", "none", "none"},
		{"glm-5.30 untouched", "glm-5.30", "none", "none"},
		{"glm-4.5 untouched", "glm-4.5", "none", "none"},
		{"deepseek untouched", "deepseek-v4-flash", "none", "none"},
		{"kimi untouched", "kimi-k2.6", "none", "none"},
		{"claude untouched", "claude-sonnet-5", "none", "none"},
		{"glm-5.3 suffix matched", "glm-5.3-turbo", "none", "low"},
	}
	for _, tc := range cases {
		if got := normalizeGlm53ReasoningEffort(tc.model, tc.effort); got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}
