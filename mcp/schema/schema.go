// Package schema derives closed JSON Schema documents from Go types.
//
// "Closed" means every object schema carries additionalProperties:false. That
// is not a stylistic preference: a tool result an agent may cite has to be a
// declared shape, and a schema that permits unknown fields permits an
// undeclared one to reach a model. See the design's D5.
//
// Maps are always rejected because a map's key space is open and cannot be
// constrained by additionalProperties:false. Channels, funcs, and untyped
// interfaces are also rejected.
package schema

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

// For derives a JSON Schema object for v's type.
func For(v any) (map[string]any, error) {
	t := reflect.TypeOf(v)
	if t == nil {
		return nil, fmt.Errorf("schema: cannot derive a schema from an untyped nil or interface")
	}
	return forTypeWithVisited(t, make(map[reflect.Type]bool))
}

func forTypeWithVisited(t reflect.Type, visited map[reflect.Type]bool) (map[string]any, error) {
	// Check for cycles
	if visited[t] {
		return nil, fmt.Errorf("schema: cycle detected in type %s", t)
	}

	switch t.Kind() {
	case reflect.Pointer:
		return forTypeWithVisited(t.Elem(), visited)
	case reflect.String:
		return map[string]any{"type": "string"}, nil
	case reflect.Bool:
		return map[string]any{"type": "boolean"}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}, nil
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}, nil
	case reflect.Slice, reflect.Array:
		items, err := forTypeWithVisited(t.Elem(), visited)
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "array", "items": items}, nil
	case reflect.Struct:
		return forStructWithVisited(t, visited)
	case reflect.Map:
		return nil, fmt.Errorf("schema: maps are not supported; a map's key space is open and cannot be closed")
	default:
		return nil, fmt.Errorf("schema: unsupported kind %s for type %s", t.Kind(), t)
	}
}

func forStructWithVisited(t reflect.Type, visited map[reflect.Type]bool) (map[string]any, error) {
	// Special-case time.Time before marking as visited
	if t == reflect.TypeOf(time.Time{}) {
		return map[string]any{"type": "string", "format": "date-time"}, nil
	}

	// Mark type as visited to detect cycles
	visited[t] = true
	defer delete(visited, t)

	props := map[string]any{}
	var required []any

	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name, opts, ok := jsonName(f)
		if !ok {
			continue
		}
		sub, err := forTypeWithVisited(f.Type, visited)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", f.Name, err)
		}
		if d := f.Tag.Get("desc"); d != "" {
			sub["description"] = d
		}
		props[name] = sub

		if !opts.omitempty && f.Type.Kind() != reflect.Pointer {
			required = append(required, name)
		}
	}

	// Error if no exported fields were found
	if len(props) == 0 {
		return nil, fmt.Errorf("schema: struct type %s has no exported fields", t)
	}

	s := map[string]any{
		"type":                 "object",
		"properties":           props,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	return s, nil
}

type fieldOpts struct{ omitempty bool }

func jsonName(f reflect.StructField) (string, fieldOpts, bool) {
	tag := f.Tag.Get("json")
	if tag == "-" {
		return "", fieldOpts{}, false
	}
	parts := strings.Split(tag, ",")
	name := parts[0]
	if name == "" {
		name = f.Name
	}
	var o fieldOpts
	for _, p := range parts[1:] {
		if p == "omitempty" {
			o.omitempty = true
		}
	}
	return name, o, true
}
