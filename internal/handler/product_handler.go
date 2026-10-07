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
	"github.com/phanuwat/product-service/internal/middleware"
	"github.com/phanuwat/product-service/internal/service"
)

// ProductHandler exposes product operations over HTTP.
type ProductHandler struct {
	productService service.ProductService
	logger         *slog.Logger
}

func NewProductHandler(productService service.ProductService, logger *slog.Logger) *ProductHandler {
	return &ProductHandler{productService: productService, logger: logger}
}

// CreateProduct godoc
//
//	@ID				createProduct
//	@Summary		Create a new product
//	@Description	Create a new product. `description` and `sale_price` are nullable.
//	@Description	A PRODUCT_CREATED audit log is recorded in the same transaction.
//	@Tags			product
//	@Accept			json
//	@Produce		json
//	@Param			X-Request-ID	header		string						false	"Correlation ID (generated when omitted, echoed in the response)"
//	@Param			X-User-ID		header		string						false	"Actor recorded in the audit log (default: anonymous)"
//	@Param			body			body		dto.CreateProductRequest	true	"Product to create"
//	@Success		201				{object}	dto.CreateProductResponse
//	@Failure		400				{object}	dto.CreateProductResponse	"INVALID_REQUEST_BODY | INVALID_NAME | INVALID_PRICE | INVALID_SALE_PRICE"
//	@Failure		500				{object}	dto.CreateProductResponse	"INTERNAL_SERVER_ERROR"
//	@Router			/product [post]
func (h *ProductHandler) CreateProduct(c *gin.Context) {
	var req dto.CreateProductRequest
	if err := decodeJSONBody(c.Request.Body, &req); err != nil {
		h.writeCreateProductError(c, apperror.New(apperror.CodeInvalidRequestBody, err.Error()))
		return
	}
	if req.Name == nil {
		h.writeCreateProductError(c, apperror.New(apperror.CodeInvalidName, "name is required"))
		return
	}
	if req.Price == nil {
		h.writeCreateProductError(c, apperror.New(apperror.CodeInvalidPrice, "price is required"))
		return
	}

	p, err := h.productService.CreateProduct(c.Request.Context(), service.CreateProductInput{
		Name:        *req.Name,
		Description: req.Description,
		Price:       *req.Price,
		SalePrice:   req.SalePrice,
	})
	if err != nil {
		h.writeCreateProductError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.CreateProductResponse{Successful: true, Data: dto.NewProductData(p)})
}

// PatchProductByID godoc
//
//	@ID				patchProductByID
//	@Summary		Partially update a product by ID
//	@Description	Only keys present in the body are overwritten.
//	@Description	Omit a key to keep its current value; send `null` to clear `description` / `sale_price`.
//	@Description	`name` and `price` may be omitted but must not be `null`.
//	@Description	A PRODUCT_UPDATED audit log (before/after) is recorded when a value actually changes.
//	@Tags			product
//	@Accept			json
//	@Produce		json
//	@Param			X-Request-ID	header		string						false	"Correlation ID (generated when omitted, echoed in the response)"
//	@Param			X-User-ID		header		string						false	"Actor recorded in the audit log (default: anonymous)"
//	@Param			id				path		int							true	"Product ID"
//	@Param			body			body		dto.PatchProductRequest		true	"Fields to update"
//	@Success		200				{object}	dto.PatchProductResponse
//	@Failure		400				{object}	dto.PatchProductResponse	"INVALID_PRODUCT_ID | INVALID_REQUEST_BODY | INVALID_NAME | INVALID_PRICE | INVALID_SALE_PRICE"
//	@Failure		404				{object}	dto.PatchProductResponse	"PRODUCT_NOT_FOUND"
//	@Failure		500				{object}	dto.PatchProductResponse	"INTERNAL_SERVER_ERROR"
//	@Router			/product/{id} [patch]
func (h *ProductHandler) PatchProductByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		h.writePatchProductError(c, apperror.New(apperror.CodeInvalidProductID, "id must be a positive integer"))
		return
	}

	var req dto.PatchProductRequest
	if err := decodeJSONBody(c.Request.Body, &req); err != nil {
		h.writePatchProductError(c, apperror.New(apperror.CodeInvalidRequestBody, err.Error()))
		return
	}

	if _, err := h.productService.PatchProductByID(c.Request.Context(), id, req.ToModel()); err != nil {
		h.writePatchProductError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.PatchProductResponse{Successful: true})
}

// decodeJSONBody requires a single JSON object body.
func decodeJSONBody(r io.Reader, v any) error {
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

func (h *ProductHandler) writeCreateProductError(c *gin.Context, err error) {
	code := h.resolveErrorCode(c, err)
	c.JSON(apperror.HTTPStatus(code), dto.CreateProductResponse{Successful: false, ErrorCode: string(code), Data: nil})
}

func (h *ProductHandler) writePatchProductError(c *gin.Context, err error) {
	code := h.resolveErrorCode(c, err)
	c.JSON(apperror.HTTPStatus(code), dto.PatchProductResponse{Successful: false, ErrorCode: string(code)})
}

// resolveErrorCode maps err to its code, exposes it to the access log and
// logs the underlying cause of unexpected (internal) errors.
func (h *ProductHandler) resolveErrorCode(c *gin.Context, err error) apperror.Code {
	code := apperror.CodeOf(err)
	c.Set(middleware.ContextKeyErrorCode, string(code))
	if code == apperror.CodeInternal {
		h.logger.ErrorContext(c.Request.Context(), "unexpected error", slog.String("route", c.FullPath()), slog.Any("error", err))
	} else {
		h.logger.DebugContext(c.Request.Context(), "request rejected", slog.String("error_code", string(code)), slog.Any("error", err))
	}
	return code
}
