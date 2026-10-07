// Package handler is the HTTP layer (a.k.a. controller): it parses requests,
// calls the service layer and writes responses.
package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/phanuwat/product-service/internal/apperror"
	"github.com/phanuwat/product-service/internal/dto"
	"github.com/phanuwat/product-service/internal/service"
)

// ProductHandler exposes product operations over HTTP.
type ProductHandler struct {
	svc    service.ProductService
	logger *slog.Logger
}

func NewProductHandler(svc service.ProductService, logger *slog.Logger) *ProductHandler {
	return &ProductHandler{svc: svc, logger: logger}
}

// Create godoc
//
//	@Summary		Create product
//	@Description	Create a new product. `description` and `sale_price` are nullable.
//	@Tags			product
//	@Accept			json
//	@Produce		json
//	@Param			body	body		dto.CreateProductRequest	true	"Product to create"
//	@Success		201		{object}	dto.CreateProductResponse
//	@Failure		400		{object}	dto.CreateProductResponse	"INVALID_REQUEST_BODY | INVALID_NAME | INVALID_PRICE | INVALID_SALE_PRICE"
//	@Failure		500		{object}	dto.CreateProductResponse	"INTERNAL_SERVER_ERROR"
//	@Router			/product [post]
func (h *ProductHandler) Create(c *gin.Context) {
	var req dto.CreateProductRequest
	if err := decodeJSON(c.Request.Body, &req); err != nil {
		h.writeCreateError(c, apperror.New(apperror.CodeInvalidRequestBody, err.Error()))
		return
	}
	if req.Name == nil {
		h.writeCreateError(c, apperror.New(apperror.CodeInvalidName, "name is required"))
		return
	}
	if req.Price == nil {
		h.writeCreateError(c, apperror.New(apperror.CodeInvalidPrice, "price is required"))
		return
	}

	p, err := h.svc.Create(c.Request.Context(), service.CreateProductInput{
		Name:        *req.Name,
		Description: req.Description,
		Price:       *req.Price,
		SalePrice:   req.SalePrice,
	})
	if err != nil {
		h.writeCreateError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.CreateProductResponse{Successful: true, Data: dto.NewProductData(p)})
}

// Patch godoc
//
//	@Summary		Patch product
//	@Description	Partially update a product. Only keys present in the body are overwritten.
//	@Description	Omit a key to keep its current value; send `null` to clear `description` / `sale_price`.
//	@Description	`name` and `price` may be omitted but must not be `null`.
//	@Tags			product
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int							true	"Product ID"
//	@Param			body	body		dto.PatchProductRequest		true	"Fields to update"
//	@Success		200		{object}	dto.BaseResponse
//	@Failure		400		{object}	dto.BaseResponse	"INVALID_PRODUCT_ID | INVALID_REQUEST_BODY | INVALID_NAME | INVALID_PRICE | INVALID_SALE_PRICE"
//	@Failure		404		{object}	dto.BaseResponse	"PRODUCT_NOT_FOUND"
//	@Failure		500		{object}	dto.BaseResponse	"INTERNAL_SERVER_ERROR"
//	@Router			/product/{id} [patch]
func (h *ProductHandler) Patch(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		h.writeBaseError(c, apperror.New(apperror.CodeInvalidProductID, "id must be a positive integer"))
		return
	}

	var req dto.PatchProductRequest
	if err := decodeJSON(c.Request.Body, &req); err != nil {
		h.writeBaseError(c, apperror.New(apperror.CodeInvalidRequestBody, err.Error()))
		return
	}

	if _, err := h.svc.Patch(c.Request.Context(), id, req.ToModel()); err != nil {
		h.writeBaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.BaseResponse{Successful: true})
}

// decodeJSON requires a single JSON object body.
func decodeJSON(r io.Reader, v any) error {
	dec := json.NewDecoder(r)
	if err := dec.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("request body is required")
		}
		return err
	}
	if dec.More() {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

func (h *ProductHandler) writeCreateError(c *gin.Context, err error) {
	code := h.logAndCode(c, err)
	c.JSON(apperror.HTTPStatus(code), dto.CreateProductResponse{Successful: false, ErrorCode: string(code), Data: nil})
}

func (h *ProductHandler) writeBaseError(c *gin.Context, err error) {
	code := h.logAndCode(c, err)
	c.JSON(apperror.HTTPStatus(code), dto.BaseResponse{Successful: false, ErrorCode: string(code)})
}

func (h *ProductHandler) logAndCode(c *gin.Context, err error) apperror.Code {
	code := apperror.CodeOf(err)
	if code == apperror.CodeInternal {
		h.logger.ErrorContext(c.Request.Context(), "request failed", "path", c.FullPath(), "error", err)
	}
	return code
}
