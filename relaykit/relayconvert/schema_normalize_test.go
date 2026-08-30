package relayconvert

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadAutomationUpdateParameters(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "automation_update_parameters.json"))
	require.NoError(t, err, "read captured schema fixture")
	var params map[string]any
	require.NoError(t, kitutil.Unmarshal(data, &params))
	return params
}

// assertNoRefs walks the normalized schema and fails on any leftover $ref or $defs.
func assertNoRefs(t *testing.T, node any) {
	t.Helper()
	switch v := node.(type) {
	case map[string]any:
		assert.NotContains(t, v, "$ref", "leftover $ref in normalized schema")
		assert.NotContains(t, v, "$defs", "leftover $defs in normalized schema")
		for _, child := range v {
			assertNoRefs(t, child)
		}
	case []any:
		for _, child := range v {
			assertNoRefs(t, child)
		}
	}
}

func TestNormalizeToolParameters_RealAutomationUpdateSchema(t *testing.T) {
	params := loadAutomationUpdateParameters(t)
	require.True(t, IsComplexToolParameters(params), "captured schema must be detected as complex")

	normalized := NormalizeToolParameters(params)
	normMap, ok := normalized.(map[string]any)
	require.True(t, ok, "normalized schema must be a map")
	assert.Equal(t, "object", normMap["type"])

	for _, key := range []string{"oneOf", "anyOf", "allOf", "enum", "const", "not"} {
		assert.NotContains(t, normMap, key, "top-level %s must be removed", key)
	}
	assertNoRefs(t, normMap)

	props, ok := normMap["properties"].(map[string]any)
	require.True(t, ok, "merged schema keeps properties")
	for _, name := range []string{"mode", "id", "name", "prompt", "rrule", "status", "kind", "destination", "model", "reasoningEffort", "projectId", "targetThreadId"} {
		assert.Contains(t, props, name, "merged property %q", name)
	}

	mode, ok := props["mode"].(map[string]any)
	require.True(t, ok)
	modeEnum, ok := mode["enum"].([]any)
	require.True(t, ok)
	gotModes := make([]string, 0, len(modeEnum))
	for _, m := range modeEnum {
		gotModes = append(gotModes, m.(string))
	}
	for _, want := range []string{"view", "create", "suggested_create", "update", "suggested_update", "delete"} {
		assert.Contains(t, gotModes, want, "merged mode enum must keep %q", want)
	}

	kind, ok := props["kind"].(map[string]any)
	require.True(t, ok)
	kindEnum, ok := kind["enum"].([]any)
	require.True(t, ok)
	gotKinds := make([]string, 0, len(kindEnum))
	for _, k := range kindEnum {
		gotKinds = append(gotKinds, k.(string))
	}
	assert.Contains(t, gotKinds, "cron")
	assert.Contains(t, gotKinds, "heartbeat")

	required, ok := normMap["required"].([]any)
	require.True(t, ok, "merged schema keeps intersection of required")
	require.Len(t, required, 1)
	assert.Equal(t, "mode", required[0], "mode is required by every union variant")
}

func TestNormalizeToolParameters_PlainSchemaUnchanged(t *testing.T) {
	plain := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"location": map[string]any{"type": "string"},
		},
		"required": []any{"location"},
	}
	assert.False(t, IsComplexToolParameters(plain))
	out := NormalizeToolParameters(plain)
	assert.True(t, reflect.ValueOf(out).Pointer() == reflect.ValueOf(plain).Pointer(), "plain schemas must be returned untouched")

	var nilParams any
	assert.Nil(t, NormalizeToolParameters(nilParams), "nil parameters untouched")
	assert.Equal(t, "not-a-map", NormalizeToolParameters("not-a-map"), "non-map parameters untouched")
}

func TestNormalizeToolParameters_ResolvesRefWithSiblingType(t *testing.T) {
	// Mirrors the Moonshot rejection: {"$ref": ...} with a sibling "type".
	params := map[string]any{
		"$defs": map[string]any{
			"str": map[string]any{"type": "string"},
		},
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{"$ref": "#/$defs/str", "type": "string"},
		},
		"required": []any{"id"},
	}
	require.True(t, IsComplexToolParameters(params))
	normalized, ok := NormalizeToolParameters(params).(map[string]any)
	require.True(t, ok)
	assertNoRefs(t, normalized)
	props := normalized["properties"].(map[string]any)
	assert.Equal(t, map[string]any{"type": "string"}, props["id"])
}

func TestNormalizeToolParameters_CircularRefTerminates(t *testing.T) {
	params := map[string]any{
		"$defs": map[string]any{
			"a": map[string]any{"$ref": "#/$defs/b"},
			"b": map[string]any{"$ref": "#/$defs/a"},
		},
		"type":     "object",
		"oneOf":    []any{map[string]any{"$ref": "#/$defs/a"}},
		"required": []any{},
	}
	normalized := NormalizeToolParameters(params)
	_, ok := normalized.(map[string]any)
	require.True(t, ok, "circular refs must not deadlock or panic")
	assertNoRefs(t, normalized)
}
