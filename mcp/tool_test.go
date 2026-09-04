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
