package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type listIn struct {
	StoreSlug string `json:"store_slug" desc:"Public store slug"`
}

type listOut struct {
	Found bool     `json:"found"`
	Items []string `json:"items"`
}

func listHandler(_ context.Context, in listIn) (listOut, error) {
	return listOut{Found: true, Items: []string{in.StoreSlug}}, nil
}

func TestRegister_DerivesBothSchemas(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, Register(r, "list_store_products", "List products.", listHandler))

	tools := r.Tools()
	require.Len(t, tools, 1)

	// D5: BOTH schemas exist. A tool with an input schema and no output schema
	// is exactly the thing OpenAPI ingestion produced, and the reason it was
	// rejected.
	require.NotNil(t, tools[0].InputSchema)
	require.NotNil(t, tools[0].OutputSchema)
	assert.Equal(t, false, tools[0].OutputSchema["additionalProperties"])
}

func TestRegister_RejectsUntypedOutput(t *testing.T) {
	r := NewRegistry()
	err := Register(r, "bad", "Untyped.", func(_ context.Context, _ listIn) (any, error) {
		return nil, nil
	})
	require.Error(t, err, "an `any` result cannot be closed, so it cannot be cited")
}

func TestRegister_RejectsDuplicateName(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, Register(r, "dup", "First.", listHandler))
	require.Error(t, Register(r, "dup", "Second.", listHandler))
}

func TestInvoke_DecodesAndReturnsTypedResult(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, Register(r, "list_store_products", "List products.", listHandler))

	out, err := r.Tools()[0].Invoke(context.Background(), json.RawMessage(`{"store_slug":"bondi"}`))
	require.NoError(t, err)
	assert.Equal(t, listOut{Found: true, Items: []string{"bondi"}}, out)
}

func TestInvoke_RejectsUnknownInputField(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, Register(r, "list_store_products", "List products.", listHandler))

	_, err := r.Tools()[0].Invoke(context.Background(), json.RawMessage(`{"store_id":"7"}`))
	require.Error(t, err, "input is a closed schema; an undeclared field is a caller error")
}

func TestTools_ReturnsDeepCopiedSchemas(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, Register(r, "list_store_products", "List products.", listHandler))

	// Get first snapshot
	tools1 := r.Tools()
	require.Len(t, tools1, 1)
	tool1 := tools1[0]

	// Mutate a nested value in the input schema's properties
	// This is where a shallow copy would fail — we need to copy nested maps
	inputProps := tool1.InputSchema["properties"].(map[string]any)
	storeSlugSchema := inputProps["store_slug"].(map[string]any)
	storeSlugSchema["description"] = "MUTATED"

	// Mutate a nested value in the output schema's properties
	outputProps := tool1.OutputSchema["properties"].(map[string]any)
	foundSchema := outputProps["found"].(map[string]any)
	foundSchema["type"] = "MUTATED"

	// Get second snapshot
	tools2 := r.Tools()
	require.Len(t, tools2, 1)
	tool2 := tools2[0]

	// Verify the second snapshot is unaffected by the mutations
	inputProps2 := tool2.InputSchema["properties"].(map[string]any)
	storeSlugSchema2 := inputProps2["store_slug"].(map[string]any)
	assert.Equal(t, "Public store slug", storeSlugSchema2["description"], "input schema was mutated by caller")

	outputProps2 := tool2.OutputSchema["properties"].(map[string]any)
	foundSchema2 := outputProps2["found"].(map[string]any)
	assert.Equal(t, "boolean", foundSchema2["type"], "output schema was mutated by caller")
}

func TestNames_IsSorted(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, Register(r, "zebra", "Z.", listHandler))
	require.NoError(t, Register(r, "apple", "A.", listHandler))
	require.NoError(t, Register(r, "mango", "M.", listHandler))

	names := r.Names()
	assert.Equal(t, []string{"apple", "mango", "zebra"}, names)
}

func TestGet_ReturnsDeepCopyAndReportsUnknown(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, Register(r, "list_store_products", "List products.", listHandler))

	_, ok := r.Get("nope")
	assert.False(t, ok, "an unknown name must not look like a registered tool")

	tool, ok := r.Get("list_store_products")
	require.True(t, ok)
	tool.InputSchema["properties"].(map[string]any)["store_slug"].(map[string]any)["description"] = "MUTATED"

	again, ok := r.Get("list_store_products")
	require.True(t, ok)
	assert.Equal(t, "Public store slug",
		again.InputSchema["properties"].(map[string]any)["store_slug"].(map[string]any)["description"],
		"Get must hand back a deep copy, not registry state")
}

// A connector has to tell "the agent sent bad arguments" from "the handler
// failed" to pick an outcome label, and string-matching a fmt.Errorf is not a
// seam.
func TestInvoke_DecodeFailureIsErrInvalidArguments(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, Register(r, "list_store_products", "List products.", listHandler))
	tool, ok := r.Get("list_store_products")
	require.True(t, ok)

	_, err := tool.Invoke(context.Background(), json.RawMessage(`{"store_id":"7"}`))
	require.ErrorIs(t, err, ErrInvalidArguments, "an undeclared field is a caller error")

	_, err = tool.Invoke(context.Background(), json.RawMessage(`{"store_slug":`))
	require.ErrorIs(t, err, ErrInvalidArguments, "malformed JSON is a caller error")
}

// The package is strict everywhere else; a second JSON value after a valid one
// must not be silently discarded.
func TestInvoke_RejectsTrailingGarbage(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, Register(r, "list_store_products", "List products.", listHandler))
	tool, _ := r.Get("list_store_products")

	_, err := tool.Invoke(context.Background(), json.RawMessage(`{"store_slug":"bondi"} {"store_slug":"evil"}`))
	require.ErrorIs(t, err, ErrInvalidArguments)
}
