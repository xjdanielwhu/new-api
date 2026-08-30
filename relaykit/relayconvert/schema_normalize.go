package relayconvert

import (
	"reflect"
	"strings"
)

// toolSchemaRefPrefix matches the JSON Schema reference style emitted by code
// generators used in newer Codex desktop builds:
//
//	{"$defs": {"__schema0": {...}}, "oneOf": [{"$ref": "#/$defs/__schema0"}, ...]}
const toolSchemaRefPrefix = "#/$defs/"

var toolSchemaTopLevelCombinators = []string{"oneOf", "anyOf", "allOf"}

// IsComplexToolParameters reports whether the given function parameters schema
// uses $defs/$ref or a top-level composition keyword (oneOf/anyOf/allOf). These
// shapes are produced by automated schema generators (for example the Codex app
// "automation_update" MCP tool) and are rejected by strict upstream validators:
// OpenAI/Anthropic forbid top-level combinators, and Moonshot rejects $ref
// entries carrying a sibling "type". Ordinary hand-written tool schemas never
// match and are left untouched.
func IsComplexToolParameters(params any) bool {
	m, ok := params.(map[string]any)
	if !ok {
		return false
	}
	if _, hasDefs := m["$defs"]; hasDefs {
		return true
	}
	for _, key := range toolSchemaTopLevelCombinators {
		if _, ok := m[key]; ok {
			return true
		}
	}
	return false
}

// NormalizeToolParameters rewrites a generated function parameters schema into a
// plain, fully inlined JSON Schema accepted by OpenAI, Anthropic and Moonshot:
//
//   - $defs entries are dropped and every $ref is resolved by inlining.
//   - A top-level oneOf/anyOf/allOf union of object variants is merged into a
//     single object schema (union of properties, intersection of required).
//   - Nested combinators inside properties are preserved (all three providers
//     accept them there).
//
// Schemas that do not use the generated shapes are returned unchanged.
func NormalizeToolParameters(params any) any {
	m, ok := params.(map[string]any)
	if !ok {
		return params
	}
	defs, _ := m["$defs"].(map[string]any)
	if len(defs) == 0 && !hasToolSchemaCombinator(m) {
		return params
	}
	resolved := resolveToolSchemaRefs(defs, m, map[string]bool{})
	resolvedMap, ok := resolved.(map[string]any)
	if !ok {
		return resolved
	}
	if hasToolSchemaCombinator(resolvedMap) {
		return mergeToolSchemaVariants(resolvedMap)
	}
	return resolvedMap
}

func hasToolSchemaCombinator(m map[string]any) bool {
	for _, key := range toolSchemaTopLevelCombinators {
		if _, ok := m[key]; ok {
			return true
		}
	}
	return false
}

func resolveToolSchemaRefs(defs map[string]any, node any, refStack map[string]bool) any {
	switch v := node.(type) {
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, resolveToolSchemaRefs(defs, item, refStack))
		}
		return out
	case map[string]any:
		ref, isRef := v["$ref"].(string)
		if isRef && strings.HasPrefix(ref, toolSchemaRefPrefix) {
			name := strings.TrimPrefix(ref, toolSchemaRefPrefix)
			target, found := defs[name]
			if found && !refStack[ref] {
				refStack[ref] = true
				resolved := resolveToolSchemaRefs(defs, target, refStack)
				delete(refStack, ref)
				if resolvedMap, ok := resolved.(map[string]any); ok {
					for key, val := range v {
						if key == "$ref" {
							continue
						}
						resolvedMap[key] = resolveToolSchemaRefs(defs, val, refStack)
					}
					return resolvedMap
				}
				return resolved
			}
			// Unresolvable or circular reference: drop $ref, keep siblings so the
			// schema remains valid JSON Schema.
			out := make(map[string]any, len(v)-1)
			for key, val := range v {
				if key == "$ref" {
					continue
				}
				out[key] = resolveToolSchemaRefs(defs, val, refStack)
			}
			return out
		}
		out := make(map[string]any, len(v))
		for key, val := range v {
			if key == "$defs" {
				continue
			}
			out[key] = resolveToolSchemaRefs(defs, val, refStack)
		}
		return out
	default:
		return node
	}
}

// mergeToolSchemaVariants merges a resolved top-level union of object variants
// into a single plain object schema. Non-object variants are skipped; if no
// object variant exists a minimal {"type":"object"} schema is returned so the
// tool call still passes strict upstream validation.
func mergeToolSchemaVariants(m map[string]any) map[string]any {
	variants := make([]map[string]any, 0, 4)
	collectToolSchemaObjectVariants(m, &variants)

	properties := make(map[string]any)
	var requiredSet map[string]struct{}
	description, _ := m["description"].(string)

	for _, variant := range variants {
		variantProps, _ := variant["properties"].(map[string]any)
		for name, propSchema := range variantProps {
			if existing, ok := properties[name]; ok {
				properties[name] = mergeToolSchemaProperty(existing, propSchema)
			} else {
				properties[name] = propSchema
			}
		}
		variantRequired, _ := variant["required"].([]any)
		if requiredSet == nil {
			requiredSet = make(map[string]struct{}, len(variantRequired))
			for _, item := range variantRequired {
				if s, ok := item.(string); ok {
					requiredSet[s] = struct{}{}
				}
			}
			continue
		}
		current := make(map[string]struct{}, len(requiredSet))
		for item := range requiredSet {
			current[item] = struct{}{}
		}
		for item := range requiredSet {
			found := false
			for _, r := range variantRequired {
				if s, ok := r.(string); ok && s == item {
					found = true
					break
				}
			}
			if !found {
				delete(current, item)
			}
		}
		requiredSet = current
	}

	merged := make(map[string]any, 3)
	merged["type"] = "object"
	if len(properties) > 0 {
		merged["properties"] = properties
	}
	if len(requiredSet) > 0 {
		required := make([]any, 0, len(requiredSet))
		for item := range requiredSet {
			required = append(required, item)
		}
		merged["required"] = required
	}
	if description != "" {
		merged["description"] = description
	}
	return merged
}

func collectToolSchemaObjectVariants(node map[string]any, out *[]map[string]any) {
	for _, key := range toolSchemaTopLevelCombinators {
		list, ok := node[key].([]any)
		if !ok {
			continue
		}
		for _, item := range list {
			sub, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if hasToolSchemaCombinator(sub) {
				collectToolSchemaObjectVariants(sub, out)
				continue
			}
			if t, _ := sub["type"].(string); t != "" && t != "object" {
				continue
			}
			*out = append(*out, sub)
		}
		return
	}
	*out = append(*out, node)
}

func mergeToolSchemaProperty(a, b any) any {
	am, aOK := a.(map[string]any)
	bm, bOK := b.(map[string]any)
	if !aOK || !bOK {
		return a
	}
	if mapsEqual(am, bm) {
		return a
	}
	if aEnum, aHas := am["enum"].([]any); aHas {
		if bEnum, bHas := bm["enum"].([]any); bHas {
			union := make([]any, 0, len(aEnum)+len(bEnum))
			seen := make(map[string]struct{}, len(union))
			for _, item := range append(append([]any{}, aEnum...), bEnum...) {
				s, ok := item.(string)
				if !ok {
					continue
				}
				if _, dup := seen[s]; dup {
					continue
				}
				seen[s] = struct{}{}
				union = append(union, s)
			}
			merged := make(map[string]any, 2)
			merged["enum"] = union
			if t, ok := am["type"].(string); ok {
				merged["type"] = t
			} else if t, ok := bm["type"].(string); ok {
				merged["type"] = t
			}
			return merged
		}
	}
	if hasNullableAnyOf(am) {
		return a
	}
	if hasNullableAnyOf(bm) {
		return b
	}
	if am["type"] == bm["type"] {
		return a
	}
	return a
}

func hasNullableAnyOf(m map[string]any) bool {
	list, ok := m["anyOf"].([]any)
	if !ok {
		return false
	}
	for _, item := range list {
		if sub, ok := item.(map[string]any); ok {
			if sub["type"] == "null" {
				return true
			}
		}
	}
	return false
}

func mapsEqual(a, b map[string]any) bool {
	return reflect.DeepEqual(a, b)
}
