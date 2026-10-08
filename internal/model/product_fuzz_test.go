package model_test

import (
	"testing"

	"github.com/get-phanuwat-api1/product-service/internal/model"
	"github.com/get-phanuwat-api1/product-service/pkg/optional"
)

func field[T any](set, null bool, v T) optional.Field[T] {
	switch {
	case !set:
		return optional.Undefined[T]()
	case null:
		return optional.Null[T]()
	default:
		return optional.Of(v)
	}
}

// FuzzProduct_ApplyPatch checks the invariants of a partial update for arbitrary patches:
// on error the product is left untouched; on success the product is valid and
// undefined fields keep their previous value.
//
//	go test ./internal/model -run '^$' -fuzz FuzzProduct_ApplyPatch -fuzztime 30s
func FuzzProduct_ApplyPatch(f *testing.F) {
	f.Add("Banana", true, false, "desc", true, true, 50.0, true, false, 40.0, true, false)
	f.Add("", true, false, "", false, false, -1.0, true, false, 0.0, false, false)
	f.Add("x", false, true, "", true, false, 100.0, false, false, 1000.0, true, false)
	f.Add("  ", true, false, "d", false, false, 0.0, true, true, 0.0, true, true)

	f.Fuzz(func(t *testing.T,
		name string, nameSet, nameNull bool,
		desc string, descSet, descNull bool,
		price float64, priceSet, priceNull bool,
		sale float64, saleSet, saleNull bool,
	) {
		original := baseProduct()
		p := baseProduct()
		patch := model.ProductPatch{
			Name:        field(nameSet, nameNull, name),
			Description: field(descSet, descNull, desc),
			Price:       field(priceSet, priceNull, price),
			SalePrice:   field(saleSet, saleNull, sale),
		}

		if err := p.ApplyPatch(patch); err != nil {
			if !p.Snapshot().Equal(original.Snapshot()) {
				t.Fatalf("product changed despite error %v: %+v", err, p.Snapshot())
			}
			return
		}

		if err := p.Validate(); err != nil {
			t.Fatalf("patched product is invalid: %v (%+v)", err, p.Snapshot())
		}
		if !nameSet && p.Name != original.Name {
			t.Fatal("undefined name was overwritten")
		}
		if !descSet && *p.Description != *original.Description {
			t.Fatal("undefined description was overwritten")
		}
		if !priceSet && p.Price != original.Price {
			t.Fatal("undefined price was overwritten")
		}
		if !saleSet && *p.SalePrice != *original.SalePrice {
			t.Fatal("undefined sale_price was overwritten")
		}
		if descSet && descNull && p.Description != nil {
			t.Fatal("description null was not applied")
		}
		if saleSet && saleNull && p.SalePrice != nil {
			t.Fatal("sale_price null was not applied")
		}
	})
}
