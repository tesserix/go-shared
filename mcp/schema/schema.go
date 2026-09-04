// Package schema derives closed JSON Schema documents from Go types.
//
// "Closed" means every object schema carries additionalProperties:false. That
// is not a stylistic preference: a tool result an agent may cite has to be a
// declared shape, and a schema that permits unknown fields permits an
// undeclared one to reach a model. See the design's D5.
package schema

import (
	"fmt"
	"reflect"
	"strings"
)

// For derives a JSON Schema object for v's type.
func For(v any) (map[string]any, error) {
	t := reflect.TypeOf(v)
	if t == nil {
		return nil, fmt.Errorf("schema: cannot derive a schema from an untyped nil or interface")
	}
	return forType(t)
}

func forType(t reflect.Type) (map[string]any, error) {
	switch t.Kind() {
	case reflect.Pointer:
		return forType(t.Elem())
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
		items, err := forType(t.Elem())
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "array", "items": items}, nil
	case reflect.Struct:
		return forStruct(t)
	default:
		return nil, fmt.Errorf("schema: unsupported kind %s for type %s", t.Kind(), t)
	}
}

func forStruct(t reflect.Type) (map[string]any, error) {
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
		sub, err := forType(f.Type)
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
