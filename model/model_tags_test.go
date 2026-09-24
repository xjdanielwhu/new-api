package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilterTokenCountTags(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"no numeric tags", "Reasoning,Tools,Open Weights", "Reasoning,Tools,Open Weights"},
		{"capability tags only", "Reasoning,Tools,Files,Vision", "Reasoning,Tools,Files,Vision"},
		{"trailing 1M", "Reasoning,Tools,1M", "Reasoning,Tools"},
		{"mixed upper and lower units", "Files,Vision,262.1K,1.1m,200k", "Files,Vision"},
		{"leading numeric tag", "200K,Reasoning", "Reasoning"},
		{"only numeric tag", "262.1K", ""},
		{"plain decimal number", "3.5,Reasoning", "Reasoning"},
		{"bare integer tag", "2048,Tools", "Tools"},
		{"parameter count kept", "72B,Reasoning", "72B,Reasoning"},
		{"words with digits kept", "Open Weights,gpt-5.4,Vision", "Open Weights,gpt-5.4,Vision"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterTokenCountTags(tt.in)
			require.Equal(t, tt.want, got)
		})
	}
}
