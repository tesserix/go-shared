package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type product struct {
	Handle string   `json:"handle" desc:"URL-safe product slug"`
	Title  string   `json:"title"`
	Price  float64  `json:"price"`
	Tags   []string `json:"tags"`
	Note   *string  `json:"note,omitempty"`
	hidden string   //nolint:unused // unexported fields must not appear
}

func TestFor_StructIsClosedAndDescribed(t *testing.T) {
	s, err := For(product{})
	require.NoError(t, err)

	assert.Equal(t, "object", s["type"])
	// A closed schema is the whole point of D5 — an agent must not receive
	// fields nobody declared.
	assert.Equal(t, false, s["additionalProperties"])

	props := s["properties"].(map[string]any)
	assert.Equal(t, "string", props["handle"].(map[string]any)["type"])
	assert.Equal(t, "URL-safe product slug", props["handle"].(map[string]any)["description"])
	assert.Equal(t, "number", props["price"].(map[string]any)["type"])
	assert.Equal(t, "array", props["tags"].(map[string]any)["type"])
	assert.Equal(t, "string", props["tags"].(map[string]any)["items"].(map[string]any)["type"])

	assert.NotContains(t, props, "hidden", "unexported fields must never be exposed")

	// omitempty and pointers mean optional; everything else is required.
	assert.ElementsMatch(t, []any{"handle", "title", "price", "tags"}, s["required"])
}

func TestFor_RejectsUntypedInterface(t *testing.T) {
	var v any
	_, err := For(v)
	require.Error(t, err, "an interface carries no schema, so it cannot be closed")
}

func TestFor_RejectsUnsupportedKind(t *testing.T) {
	_, err := For(make(chan int))
	require.Error(t, err)
}
