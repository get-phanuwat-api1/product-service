package optional_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/get-phanuwat-api1/product-service/pkg/optional"
)

type payload struct {
	Name  optional.Field[string]  `json:"name"`
	Price optional.Field[float64] `json:"price"`
}

func TestField_UnmarshalJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		body      string
		wantName  optional.Field[string]
		wantPrice optional.Field[float64]
	}{
		{
			name:      "undefined when key absent",
			body:      `{}`,
			wantName:  optional.Undefined[string](),
			wantPrice: optional.Undefined[float64](),
		},
		{
			name:      "null when key is null",
			body:      `{"name": null, "price": null}`,
			wantName:  optional.Null[string](),
			wantPrice: optional.Null[float64](),
		},
		{
			name:      "value when key has value",
			body:      `{"name": "apple", "price": 10.5}`,
			wantName:  optional.Of("apple"),
			wantPrice: optional.Of(10.5),
		},
		{
			name:      "mixed",
			body:      `{"name": "apple"}`,
			wantName:  optional.Of("apple"),
			wantPrice: optional.Undefined[float64](),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var p payload
			require.NoError(t, json.Unmarshal([]byte(tt.body), &p))
			assert.Equal(t, tt.wantName, p.Name)
			assert.Equal(t, tt.wantPrice, p.Price)
		})
	}
}

func TestField_UnmarshalJSON_TypeMismatch(t *testing.T) {
	t.Parallel()
	var p payload
	err := json.Unmarshal([]byte(`{"price": "abc"}`), &p)
	assert.Error(t, err)
}

func TestField_Ptr(t *testing.T) {
	t.Parallel()
	assert.Nil(t, optional.Undefined[string]().Ptr())
	assert.Nil(t, optional.Null[string]().Ptr())
	got := optional.Of("x").Ptr()
	require.NotNil(t, got)
	assert.Equal(t, "x", *got)
}

func TestField_MarshalJSON(t *testing.T) {
	t.Parallel()
	b, err := json.Marshal(payload{Name: optional.Of("a"), Price: optional.Null[float64]()})
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"a","price":null}`, string(b))
}
