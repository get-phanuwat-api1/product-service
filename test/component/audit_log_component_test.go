package component

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/get-phanuwat-api1/product-service/internal/model"
	"github.com/get-phanuwat-api1/product-service/internal/repository"
	"github.com/get-phanuwat-api1/product-service/test/testutil"
)

type httpResult struct {
	status    int
	requestID string
	body      apiResponse
}

func doAs(t *testing.T, method, path, body, userID, requestID string) httpResult {
	t.Helper()
	req, err := http.NewRequest(method, server.URL+path, bytes.NewBufferString(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if userID != "" {
		req.Header.Set("X-User-ID", userID)
	}
	if requestID != "" {
		req.Header.Set("X-Request-ID", requestID)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var out apiResponse
	require.NoError(t, json.Unmarshal(raw, &out), string(raw))
	return httpResult{status: resp.StatusCode, requestID: resp.Header.Get("X-Request-ID"), body: out}
}

func auditLogs(t *testing.T, productID int64) []model.AuditLog {
	t.Helper()
	logs, err := repository.NewAuditLogRepository(pool).
		ListAuditLogsByEntity(context.Background(), model.AuditEntityProduct, productID)
	require.NoError(t, err)
	return logs
}

func TestAuditLog_CreateAndPatchAreRecorded(t *testing.T) {
	testutil.TruncateTables(t, pool)

	created := doAs(t, http.MethodPost, "/product",
		`{"name":"Apple","description":"fresh","sale_price":90,"price":100}`, "alice", "req-create-1")
	require.Equal(t, http.StatusCreated, created.status)
	assert.Equal(t, "req-create-1", created.requestID, "request id is echoed back")

	var p productData
	require.NoError(t, json.Unmarshal(created.body.Data, &p))

	patched := doAs(t, http.MethodPatch, fmt.Sprintf("/product/%d", p.ID),
		`{"name":"Banana","sale_price":null}`, "bob", "req-patch-1")
	require.Equal(t, http.StatusOK, patched.status)

	logs := auditLogs(t, p.ID)
	require.Len(t, logs, 2)

	assert.Equal(t, model.AuditActionProductCreated, logs[0].Action)
	assert.Equal(t, "alice", logs[0].Actor)
	assert.Equal(t, "req-create-1", logs[0].RequestID)
	assert.Nil(t, logs[0].BeforeData)
	assert.JSONEq(t, fmt.Sprintf(`{"id":%d,"name":"Apple","description":"fresh","price":100,"sale_price":90}`, p.ID),
		string(logs[0].AfterData))

	assert.Equal(t, model.AuditActionProductUpdated, logs[1].Action)
	assert.Equal(t, "bob", logs[1].Actor)
	assert.Equal(t, "req-patch-1", logs[1].RequestID)
	assert.JSONEq(t, fmt.Sprintf(`{"id":%d,"name":"Apple","description":"fresh","price":100,"sale_price":90}`, p.ID),
		string(logs[1].BeforeData))
	assert.JSONEq(t, fmt.Sprintf(`{"id":%d,"name":"Banana","description":"fresh","price":100,"sale_price":null}`, p.ID),
		string(logs[1].AfterData))
}

func TestAuditLog_DefaultsWhenHeadersMissing(t *testing.T) {
	testutil.TruncateTables(t, pool)

	res := doAs(t, http.MethodPost, "/product", `{"name":"Apple","price":1}`, "", "")
	require.Equal(t, http.StatusCreated, res.status)
	require.Len(t, res.requestID, 32, "request id is generated")

	var p productData
	require.NoError(t, json.Unmarshal(res.body.Data, &p))
	logs := auditLogs(t, p.ID)
	require.Len(t, logs, 1)
	assert.Equal(t, "anonymous", logs[0].Actor)
	assert.Equal(t, res.requestID, logs[0].RequestID)
}

func TestAuditLog_NotRecordedForFailedOrNoOpRequests(t *testing.T) {
	testutil.TruncateTables(t, pool)
	p := createProduct(t, `{"name":"Apple","description":"fresh","sale_price":90,"price":100}`)
	path := fmt.Sprintf("/product/%d", p.ID)

	for _, body := range []string{
		`{"price":-1}`,                 // validation error
		`{"name":null}`,                // validation error
		`{}`,                           // empty patch
		`{"name":"Apple","price":100}`, // same values
		`{"description":"fresh"}`,      // same value
	} {
		res := doAs(t, http.MethodPatch, path, body, "carol", "")
		require.Contains(t, []int{http.StatusOK, http.StatusBadRequest}, res.status, body)
	}

	logs := auditLogs(t, p.ID)
	require.Len(t, logs, 1, "only the create is audited")
	assert.Equal(t, model.AuditActionProductCreated, logs[0].Action)
}
