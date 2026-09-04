package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"sync"

	"github.com/tesserix/go-shared/mcp/schema"
)

// Tool is a registered, fully-described tool.
type Tool struct {
	Name         string
	Description  string
	InputSchema  map[string]any
	OutputSchema map[string]any
	Invoke       func(context.Context, json.RawMessage) (any, error)
}

// Registry holds the tools a server serves. Safe for concurrent registration and
// reading. Returned tools carry their own deep copies of their schemas, so a
// caller cannot mutate registry state through a returned Tool.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}}
}

// Register adds a tool, deriving both schemas from the handler's own signature.
//
// The generic signature is the enforcement of D5, not a convenience: there is
// no way to call this without an input type and an output type, so a tool
// cannot exist without both. The only remaining hole is naming `any` as the
// output, which is rejected below.
func Register[In, Out any](r *Registry, name, description string, h func(context.Context, In) (Out, error)) error {
	if name == "" {
		return fmt.Errorf("mcp: tool name must not be empty")
	}
	if description == "" {
		return fmt.Errorf("mcp: tool %q must carry a description — it is what the model reads", name)
	}

	var out Out
	if reflect.TypeOf(&out).Elem().Kind() == reflect.Interface {
		return fmt.Errorf("mcp: tool %q declares an interface result; an untyped result cannot be closed or cited", name)
	}

	var in In
	inSchema, err := schema.For(in)
	if err != nil {
		return fmt.Errorf("mcp: tool %q input: %w", name, err)
	}
	outSchema, err := schema.For(out)
	if err != nil {
		return fmt.Errorf("mcp: tool %q output: %w", name, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("mcp: tool %q is already registered", name)
	}

	r.tools[name] = Tool{
		Name:         name,
		Description:  description,
		InputSchema:  inSchema,
		OutputSchema: outSchema,
		Invoke: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var args In
			dec := json.NewDecoder(bytes.NewReader(raw))
			// The input schema is closed, so the decoder must be too —
			// otherwise a caller can pass a field the schema forbids and be
			// silently ignored rather than corrected.
			dec.DisallowUnknownFields()
			if len(raw) > 0 {
				if err := dec.Decode(&args); err != nil {
					return nil, fmt.Errorf("mcp: tool %q arguments: %w", name, err)
				}
			}
			return h(ctx, args)
		},
	}
	return nil
}

// deepCopySchema recursively deep-copies a JSON Schema map, including nested
// maps and slices, so that returned tools carry independent schema copies.
func deepCopySchema(s map[string]any) map[string]any {
	if s == nil {
		return nil
	}
	result := make(map[string]any, len(s))
	for k, v := range s {
		result[k] = deepCopyValue(v)
	}
	return result
}

// deepCopyValue recursively copies a value, handling maps and slices.
func deepCopyValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return deepCopySchema(val)
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = deepCopyValue(item)
		}
		return result
	default:
		// Scalars (string, bool, number, nil) are immutable and safe to share
		return v
	}
}

// Tools returns every registered tool, sorted by name. Each tool carries its own
// deep copy of its schemas, so mutations by the caller do not affect registry state
// or other tools.
func (r *Registry) Tools() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		// Deep-copy schemas so returned tools are independent of registry state
		toolCopy := Tool{
			Name:         t.Name,
			Description:  t.Description,
			InputSchema:  deepCopySchema(t.InputSchema),
			OutputSchema: deepCopySchema(t.OutputSchema),
			Invoke:       t.Invoke,
		}
		out = append(out, toolCopy)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Names returns registered tool names, sorted. The declared-vs-served check
// compares this against the registry record.
func (r *Registry) Names() []string {
	tools := r.Tools()
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	return names
}
