package services

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestResolveReferences_ListResolvesToArray(t *testing.T) {
	gvMap := map[string]map[string]any{
		"net": {"hosts": []any{"a.internal", "b.internal"}},
	}
	payload := json.RawMessage(`{"allowedHosts": "${net.hosts}"}`)

	resolved, rerr := ResolveReferences(payload, gvMap)
	if rerr != nil {
		t.Fatalf("unexpected render error: %v", rerr)
	}

	want := []any{"a.internal", "b.internal"}
	if !reflect.DeepEqual(resolved["allowedHosts"], want) {
		t.Fatalf("expected list to resolve to %#v, got %#v", want, resolved["allowedHosts"])
	}
}

func TestResolveReferences_EmbeddedListRefIsError(t *testing.T) {
	gvMap := map[string]map[string]any{
		"net": {"hosts": []any{"a.internal", "b.internal"}},
	}
	payload := json.RawMessage(`{"label": "prefix-${net.hosts}"}`)

	_, rerr := ResolveReferences(payload, gvMap)
	if rerr == nil {
		t.Fatal("expected a render error for an embedded list reference, got nil")
	}
	if rerr.Kind != ErrNonScalarRef {
		t.Fatalf("expected error kind %q, got %q", ErrNonScalarRef, rerr.Kind)
	}
}
