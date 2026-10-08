package optional_test

import (
	"encoding/json"
	"testing"

	"github.com/get-phanuwat-api1/product-service/pkg/optional"
)

// FuzzField_MatchesPointerDecoding checks that, for any JSON payload, Field[T] decodes exactly
// like encoding/json decodes into a *T, plus the extra "was the key present" information.
//
//	go test ./pkg/optional -run '^$' -fuzz FuzzField_MatchesPointerDecoding -fuzztime 30s
func FuzzField_MatchesPointerDecoding(f *testing.F) {
	for _, seed := range []string{
		`{}`, `{"v":null}`, `{"v":"x"}`, `{"v":""}`, `{"v":1}`, `{"v":true}`,
		`{"v":"\u0000"}`, `{"v":"a","v":null}`, `{"v":null,"v":"a"}`, `{"V":"case"}`, `null`, `[]`, `{"v":`,
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		var got struct {
			V optional.Field[string] `json:"v"`
		}
		var want struct {
			V *string `json:"v"`
		}
		var keys map[string]json.RawMessage

		gotErr := json.Unmarshal(data, &got)
		wantErr := json.Unmarshal(data, &want)
		if (gotErr == nil) != (wantErr == nil) {
			t.Fatalf("error mismatch: field=%v pointer=%v", gotErr, wantErr)
		}
		if gotErr != nil {
			return
		}

		// value semantics must match pointer decoding
		if (want.V == nil) != !got.V.HasValue() {
			t.Fatalf("presence mismatch: field=%+v pointer=%v", got.V, want.V)
		}
		if want.V != nil && *want.V != got.V.Value {
			t.Fatalf("value mismatch: field=%q pointer=%q", got.V.Value, *want.V)
		}
		if got.V.Null && got.V.Value != "" {
			t.Fatalf("null field must hold the zero value, got %q", got.V.Value)
		}

		// Set must be true iff the key is present (case-insensitive, like encoding/json)
		if json.Unmarshal(data, &keys) == nil {
			present := false
			for k := range keys {
				if equalFold(k, "v") {
					present = true
				}
			}
			if present != got.V.Set {
				t.Fatalf("Set=%v but key present=%v in %s", got.V.Set, present, data)
			}
		}

		// round trip: marshal then unmarshal keeps the value
		out, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var back struct {
			V optional.Field[string] `json:"v"`
		}
		if err := json.Unmarshal(out, &back); err != nil {
			t.Fatalf("unmarshal round trip %s: %v", out, err)
		}
		if back.V.HasValue() != got.V.HasValue() || back.V.Value != got.V.Value {
			t.Fatalf("round trip changed value: %+v -> %+v", got.V, back.V)
		}
	})
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}
