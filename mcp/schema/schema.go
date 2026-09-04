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
//
// Embedded structs follow encoding/json: an anonymous field with no json name
// is flattened into the parent, and one with a json name stays a nested object.
// The shapes that cannot be described faithfully — an embedded pointer or
// interface, or a promoted name that collides with a declared one — are errors
// at derivation time rather than a schema that disagrees with the wire.
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
		name, opts, ok := jsonName(f)
		if !ok {
			continue
		}
		// An anonymous field with no json name is PROMOTED by encoding/json:
		// its fields are marshalled into the parent object, not under the
		// embedded type's name. The schema has to say the same thing, or every
		// result of such a tool violates its own additionalProperties:false
		// and the decoder accepts what the schema forbids.
		if f.Anonymous && !hasJSONName(f) {
			switch f.Type.Kind() {
			case reflect.Struct:
				if err := flattenEmbedded(f, props, &required, visited); err != nil {
					return nil, err
				}
				continue
			case reflect.Pointer:
				return nil, fmt.Errorf("schema: embedded pointer field %s (%s) is not supported: encoding/json promotes its fields, but whether they appear at all depends on the pointer being non-nil, which a static schema cannot express — give it an explicit json tag or inline its fields", f.Name, f.Type)
			case reflect.Interface:
				return nil, fmt.Errorf("schema: embedded interface field %s (%s) is not supported: its fields are not known until runtime, so the schema cannot be closed", f.Name, f.Type)
			}
			// Any other embedded kind (a named scalar, say) is treated by
			// encoding/json as an ordinary field named after its type, which
			// is what the code below does.
		}
		// An embedded struct type may itself be unexported and still contribute
		// to the wire — encoding/json keeps it precisely because its fields may
		// be exported. Every other unexported field is invisible to json.
		if !f.IsExported() && !(f.Anonymous && f.Type.Kind() == reflect.Struct) {
			continue
		}
		sub, err := forTypeWithVisited(f.Type, visited)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", f.Name, err)
		}
		if d := f.Tag.Get("desc"); d != "" {
			sub["description"] = d
		}
		if _, exists := props[name]; exists {
			return nil, fmt.Errorf("schema: type %s declares property %q twice; encoding/json resolves same-depth duplicates by dropping both, and a promoted-vs-declared clash by depth — neither is expressible in one schema, so give one of them a distinct json tag", t, name)
		}
		props[name] = sub

		if !opts.omitempty && f.Type.Kind() != reflect.Pointer {
			required = append(required, name)
		}
	}

	// Error if struct has fields but none are exported (e.g. time.Time-like types)
	// Allow struct{} through as it correctly represents "no arguments"
	if len(props) == 0 && t.NumField() > 0 {
		return nil, fmt.Errorf("schema: struct type %s has fields but none are exported, so its schema would match nothing", t)
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

// flattenEmbedded merges an untagged embedded struct's properties and required
// entries into the parent, exactly as encoding/json promotes its fields.
func flattenEmbedded(f reflect.StructField, props map[string]any, required *[]any, visited map[reflect.Type]bool) error {
	sub, err := forStructWithVisited(f.Type, visited)
	if err != nil {
		return fmt.Errorf("embedded field %s: %w", f.Name, err)
	}
	subProps, ok := sub["properties"].(map[string]any)
	if !ok {
		// e.g. an embedded time.Time, which derives a string schema and whose
		// MarshalJSON replaces the whole parent object on the wire.
		return fmt.Errorf("schema: embedded field %s (%s) does not derive an object schema, so its fields cannot be promoted; give it an explicit json tag", f.Name, f.Type)
	}
	for k, v := range subProps {
		if _, exists := props[k]; exists {
			return fmt.Errorf("schema: embedded field %s promotes property %q, which %s already declares; encoding/json picks the shallower one by depth and the schema cannot express that — give one of them a distinct json tag", f.Name, k, f.Type)
		}
		props[k] = v
	}
	if r, ok := sub["required"].([]any); ok {
		*required = append(*required, r...)
	}
	return nil
}

type fieldOpts struct{ omitempty bool }

// hasJSONName reports whether f carries an explicit name in its json tag. An
// anonymous field with one is an ordinary named field to encoding/json, not a
// promoted embed.
func hasJSONName(f reflect.StructField) bool {
	name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	return name != ""
}

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
