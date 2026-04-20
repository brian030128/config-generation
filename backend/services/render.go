package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"text/template"

	sprig "github.com/Masterminds/sprig/v3"
)

// RenderErrorKind classifies config generation errors.
type RenderErrorKind string

const (
	ErrMissingValues       RenderErrorKind = "missing_values"
	ErrUnknownGlobalValues RenderErrorKind = "unknown_global_values"
	ErrUnknownKey          RenderErrorKind = "unknown_key"
	ErrTemplateParse       RenderErrorKind = "template_parse"
	ErrTemplateExec        RenderErrorKind = "template_exec"
)

// RenderError is a structured error returned by the rendering pipeline.
type RenderError struct {
	Kind             RenderErrorKind `json:"kind"`
	TemplateName     string          `json:"template_name,omitempty"`
	GlobalValuesName string          `json:"global_values_name,omitempty"`
	KeyName          string          `json:"key_name,omitempty"`
	Message          string          `json:"message"`
}

func (e *RenderError) Error() string {
	return e.Message
}

// TemplateInput is a template to be rendered.
type TemplateInput struct {
	Name string
	Body string
}

// RenderResult is the outcome of rendering a single template.
type RenderResult struct {
	TemplateName   string       `json:"template_name"`
	RenderedOutput *string      `json:"rendered_output,omitempty"`
	Error          *RenderError `json:"error,omitempty"`
}

// refPattern matches ${name.key} references in value strings.
var refPattern = regexp.MustCompile(`\$\{(\w+)\.(\w+)\}`)

// ExtractGlobalValueRefs scans a values payload for ${name.key} references
// and returns the deduplicated set of global values names referenced.
func ExtractGlobalValueRefs(payload json.RawMessage) ([]string, error) {
	var data any
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	walkForRefs(data, seen)
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	return names, nil
}

func walkForRefs(v any, seen map[string]bool) {
	switch val := v.(type) {
	case string:
		matches := refPattern.FindAllStringSubmatch(val, -1)
		for _, m := range matches {
			seen[m[1]] = true
		}
	case map[string]any:
		for _, child := range val {
			walkForRefs(child, seen)
		}
	case []any:
		for _, child := range val {
			walkForRefs(child, seen)
		}
	}
}

// ResolveReferences deep-walks a values payload and replaces every ${name.key}
// reference with the corresponding scalar from globalValuesMap.
// globalValuesMap is keyed by global values name → flat key-value map.
func ResolveReferences(payload json.RawMessage, globalValuesMap map[string]map[string]any) (map[string]any, *RenderError) {
	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, &RenderError{
			Kind:    ErrMissingValues,
			Message: fmt.Sprintf("failed to parse values payload: %s", err),
		}
	}

	resolved, rerr := resolveValue(data, globalValuesMap)
	if rerr != nil {
		return nil, rerr
	}
	return resolved.(map[string]any), nil
}

func resolveValue(v any, gvMap map[string]map[string]any) (any, *RenderError) {
	switch val := v.(type) {
	case string:
		return resolveString(val, gvMap)
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, child := range val {
			resolved, err := resolveValue(child, gvMap)
			if err != nil {
				return nil, err
			}
			result[k] = resolved
		}
		return result, nil
	case []any:
		result := make([]any, len(val))
		for i, child := range val {
			resolved, err := resolveValue(child, gvMap)
			if err != nil {
				return nil, err
			}
			result[i] = resolved
		}
		return result, nil
	default:
		return v, nil
	}
}

func resolveString(s string, gvMap map[string]map[string]any) (any, *RenderError) {
	// Check if the entire string is a single reference — return the scalar directly
	// (preserves type: numbers stay numbers, bools stay bools)
	if match := refPattern.FindStringSubmatch(s); match != nil && match[0] == s {
		name, key := match[1], match[2]
		gv, ok := gvMap[name]
		if !ok {
			return nil, &RenderError{
				Kind:             ErrUnknownGlobalValues,
				GlobalValuesName: name,
				Message:          fmt.Sprintf("unknown global values entry %q", name),
			}
		}
		val, ok := gv[key]
		if !ok {
			return nil, &RenderError{
				Kind:             ErrUnknownKey,
				GlobalValuesName: name,
				KeyName:          key,
				Message:          fmt.Sprintf("unknown key %q in global values %q", key, name),
			}
		}
		return val, nil
	}

	// Otherwise, do inline replacement for embedded references (${a.b} within larger strings)
	var firstErr *RenderError
	result := refPattern.ReplaceAllStringFunc(s, func(match string) string {
		if firstErr != nil {
			return match
		}
		parts := refPattern.FindStringSubmatch(match)
		name, key := parts[1], parts[2]
		gv, ok := gvMap[name]
		if !ok {
			firstErr = &RenderError{
				Kind:             ErrUnknownGlobalValues,
				GlobalValuesName: name,
				Message:          fmt.Sprintf("unknown global values entry %q", name),
			}
			return match
		}
		val, ok := gv[key]
		if !ok {
			firstErr = &RenderError{
				Kind:             ErrUnknownKey,
				GlobalValuesName: name,
				KeyName:          key,
				Message:          fmt.Sprintf("unknown key %q in global values %q", key, name),
			}
			return match
		}
		return fmt.Sprintf("%v", val)
	})
	if firstErr != nil {
		return nil, firstErr
	}
	return result, nil
}

// RenderTemplate parses and executes a Go template with Sprig functions.
func RenderTemplate(name string, body string, data map[string]any) (string, *RenderError) {
	tmpl, err := template.New(name).Funcs(sprig.TxtFuncMap()).Parse(body)
	if err != nil {
		return "", &RenderError{
			Kind:         ErrTemplateParse,
			TemplateName: name,
			Message:      fmt.Sprintf("template parse error in %q: %s", name, err),
		}
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", &RenderError{
			Kind:         ErrTemplateExec,
			TemplateName: name,
			Message:      fmt.Sprintf("template execution error in %q: %s", name, err),
		}
	}
	return buf.String(), nil
}

// RenderAll resolves references once then renders each template.
// Returns a result per template — either rendered output or error.
func RenderAll(templates []TemplateInput, valuesPayload json.RawMessage, globalValuesMap map[string]map[string]any) []RenderResult {
	resolved, resolveErr := ResolveReferences(valuesPayload, globalValuesMap)

	results := make([]RenderResult, len(templates))
	for i, t := range templates {
		results[i].TemplateName = t.Name

		if resolveErr != nil {
			errCopy := *resolveErr
			errCopy.TemplateName = t.Name
			results[i].Error = &errCopy
			continue
		}

		output, renderErr := RenderTemplate(t.Name, t.Body, resolved)
		if renderErr != nil {
			results[i].Error = renderErr
		} else {
			results[i].RenderedOutput = &output
		}
	}
	return results
}
