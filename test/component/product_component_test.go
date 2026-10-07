// Package component runs end-to-end tests within the service boundary:
// real HTTP router + handler + service + repository + PostgreSQL (testcontainers).
package component

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phanuwat/product-service/internal/container"
	"github.com/phanuwat/product-service/pkg/logger"
	"github.com/phanuwat/product-service/test/testutil"
)

var (
	pool   *pgxpool.Pool
	server *httptest.Server
)

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		fmt.Println("skipping component tests in -short mode")
		os.Exit(0)
	}
	gin.SetMode(gin.TestMode)

	ctx := context.Background()
	p, cleanup, err := testutil.StartPostgres(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot start postgres (is Docker running?): %v\n", err)
		os.Exit(1)
	}
	pool = p
	server = httptest.NewServer(container.NewContainer(pool, logger.NewDiscardLogger()).Router)

	code := m.Run()
	server.Close()
	cleanup()
	os.Exit(code)
}

type apiResponse struct {
	Successful bool            `json:"successful"`
	ErrorCode  string          `json:"error_code"`
	Data       json.RawMessage `json:"data"`
}

type productData struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description *string  `json:"description"`
	Price       float64  `json:"price"`
	SalePrice   *float64 `json:"sale_price"`
}

func do(t *testing.T, method, path, body string) (int, apiResponse, map[string]json.RawMessage) {
	t.Helper()
	req, err := http.NewRequest(method, server.URL+path, bytes.NewBufferString(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var out apiResponse
	require.NoError(t, json.Unmarshal(raw, &out), string(raw))
	var keys map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &keys))
	return resp.StatusCode, out, keys
}

func createProduct(t *testing.T, body string) productData {
	t.Helper()
	status, resp, _ := do(t, http.MethodPost, "/product", body)
	require.Equal(t, http.StatusCreated, status)
	require.True(t, resp.Successful)
	var p productData
	require.NoError(t, json.Unmarshal(resp.Data, &p))
	return p
}

func loadProduct(t *testing.T, id int64) productData {
	t.Helper()
	var p productData
	err := pool.QueryRow(context.Background(),
		`SELECT id, name, description, price, sale_price FROM products WHERE id = $1`, id).
		Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.SalePrice)
	require.NoError(t, err)
	return p
}

func TestCreateProduct(t *testing.T) {
	testutil.TruncateTables(t, pool)

	t.Run("full body", func(t *testing.T) {
		status, resp, keys := do(t, http.MethodPost, "/product",
			`{"name":"iPhone","description":"phone","sale_price":900.5,"price":1000}`)

		assert.Equal(t, http.StatusCreated, status)
		assert.True(t, resp.Successful)
		assert.Equal(t, "", resp.ErrorCode)
		assert.Contains(t, keys, "error_code")

		var p productData
		require.NoError(t, json.Unmarshal(resp.Data, &p))
		assert.NotZero(t, p.ID)
		assert.Equal(t, "iPhone", p.Name)
		assert.Equal(t, "phone", *p.Description)
		assert.Equal(t, 900.5, *p.SalePrice)
		assert.Equal(t, 1000.0, p.Price)

		assert.Equal(t, p, loadProduct(t, p.ID))
	})

	t.Run("nullable fields null or omitted", func(t *testing.T) {
		for _, body := range []string{
			`{"name":"A","price":1,"description":null,"sale_price":null}`,
			`{"name":"A","price":1}`,
		} {
			p := createProduct(t, body)
			db := loadProduct(t, p.ID)
			assert.Nil(t, db.Description)
			assert.Nil(t, db.SalePrice)
		}
	})

	errorCases := []struct {
		name string
		body string
		code string
	}{
		{"malformed json", `{"name":`, "INVALID_REQUEST_BODY"},
		{"empty body", ``, "INVALID_REQUEST_BODY"},
		{"wrong type", `{"name":"A","price":"10"}`, "INVALID_REQUEST_BODY"},
		{"missing name", `{"price":10}`, "INVALID_NAME"},
		{"null name", `{"name":null,"price":10}`, "INVALID_NAME"},
		{"blank name", `{"name":"  ","price":10}`, "INVALID_NAME"},
		{"missing price", `{"name":"A"}`, "INVALID_PRICE"},
		{"negative price", `{"name":"A","price":-1}`, "INVALID_PRICE"},
		{"sale above price", `{"name":"A","price":10,"sale_price":11}`, "INVALID_SALE_PRICE"},
	}
	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			status, resp, keys := do(t, http.MethodPost, "/product", tc.body)
			assert.Equal(t, http.StatusBadRequest, status)
			assert.False(t, resp.Successful)
			assert.Equal(t, tc.code, resp.ErrorCode)
			assert.Equal(t, "null", string(keys["data"]))
		})
	}
}

func TestPatchProduct(t *testing.T) {
	testutil.TruncateTables(t, pool)
	const seed = `{"name":"Apple","description":"fresh","sale_price":90,"price":100}`

	tests := []struct {
		name string
		body string
		want func(p productData) productData
	}{
		{
			name: "only name overwritten",
			body: `{"name":"Banana"}`,
			want: func(p productData) productData { p.Name = "Banana"; return p },
		},
		{
			name: "only price overwritten",
			body: `{"price":150}`,
			want: func(p productData) productData { p.Price = 150; return p },
		},
		{
			name: "description set to null",
			body: `{"description":null}`,
			want: func(p productData) productData { p.Description = nil; return p },
		},
		{
			name: "sale_price set to null",
			body: `{"sale_price":null}`,
			want: func(p productData) productData { p.SalePrice = nil; return p },
		},
		{
			name: "description and sale_price updated",
			body: `{"description":"ripe","sale_price":80}`,
			want: func(p productData) productData {
				d, s := "ripe", 80.0
				p.Description, p.SalePrice = &d, &s
				return p
			},
		},
		{
			name: "empty body object changes nothing",
			body: `{}`,
			want: func(p productData) productData { return p },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created := createProduct(t, seed)

			status, resp, keys := do(t, http.MethodPatch, fmt.Sprintf("/product/%d", created.ID), tt.body)

			assert.Equal(t, http.StatusOK, status)
			assert.True(t, resp.Successful)
			assert.Equal(t, "", resp.ErrorCode)
			assert.NotContains(t, keys, "data")
			assert.Equal(t, tt.want(created), loadProduct(t, created.ID))
		})
	}
}

func TestPatchProduct_Errors(t *testing.T) {
	testutil.TruncateTables(t, pool)
	created := createProduct(t, `{"name":"Apple","description":"fresh","sale_price":90,"price":100}`)
	path := fmt.Sprintf("/product/%d", created.ID)

	tests := []struct {
		name   string
		path   string
		body   string
		status int
		code   string
	}{
		{"not found", "/product/999999", `{"name":"X"}`, http.StatusNotFound, "PRODUCT_NOT_FOUND"},
		{"non numeric id", "/product/abc", `{"name":"X"}`, http.StatusBadRequest, "INVALID_PRODUCT_ID"},
		{"zero id", "/product/0", `{"name":"X"}`, http.StatusBadRequest, "INVALID_PRODUCT_ID"},
		{"malformed json", path, `{"name"`, http.StatusBadRequest, "INVALID_REQUEST_BODY"},
		{"name null", path, `{"name":null}`, http.StatusBadRequest, "INVALID_NAME"},
		{"price null", path, `{"price":null}`, http.StatusBadRequest, "INVALID_PRICE"},
		{"price below sale price", path, `{"price":50}`, http.StatusBadRequest, "INVALID_SALE_PRICE"},
		{"negative sale price", path, `{"sale_price":-1}`, http.StatusBadRequest, "INVALID_SALE_PRICE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, resp, _ := do(t, http.MethodPatch, tt.path, tt.body)
			assert.Equal(t, tt.status, status)
			assert.False(t, resp.Successful)
			assert.Equal(t, tt.code, resp.ErrorCode)
		})
	}

	// product must be untouched after all the failed patches
	assert.Equal(t, created, loadProduct(t, created.ID))
}

func TestSwaggerDocs(t *testing.T) {
	resp, err := http.Get(server.URL + "/api-docs")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "/api-docs/index.html", resp.Request.URL.Path)

	resp2, err := http.Get(server.URL + "/api-docs/doc.json")
	require.NoError(t, err)
	defer resp2.Body.Close()
	body, _ := io.ReadAll(resp2.Body)
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	assert.Contains(t, string(body), `"/product/{id}"`)
}
