package apperror_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/phanuwat/product-service/internal/apperror"
)

func TestErrorIs(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("wrapped: %w", apperror.New(apperror.CodeProductNotFound, "other message"))
	assert.ErrorIs(t, err, apperror.ErrProductNotFound)
	assert.NotErrorIs(t, apperror.New(apperror.CodeInvalidName, "x"), apperror.ErrProductNotFound)
}

func TestCodeOf(t *testing.T) {
	t.Parallel()
	assert.Equal(t, apperror.CodeInvalidName, apperror.CodeOf(apperror.New(apperror.CodeInvalidName, "x")))
	assert.Equal(t, apperror.CodeInternal, apperror.CodeOf(errors.New("boom")))
}

func TestHTTPStatus(t *testing.T) {
	t.Parallel()
	assert.Equal(t, http.StatusNotFound, apperror.HTTPStatus(apperror.CodeProductNotFound))
	assert.Equal(t, http.StatusInternalServerError, apperror.HTTPStatus(apperror.CodeInternal))
	assert.Equal(t, http.StatusBadRequest, apperror.HTTPStatus(apperror.CodeInvalidPrice))
}
