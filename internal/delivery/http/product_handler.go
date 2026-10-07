package http

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/phanuwat/product-service/internal/domain"
	"github.com/phanuwat/product-service/internal/usecase"
)

// ProductHandler exposes product usecases over HTTP.
type ProductHandler struct {
	uc     usecase.ProductUsecase
	logger *slog.Logger
}

func NewProductHandler(uc usecase.ProductUsecase, logger *slog.Logger) *ProductHandler {
	return &ProductHandler{uc: uc, logger: logger}
}

// Create godoc
//
//	@Summary		Create product
//	@Description	Create a new product. `description` and `sale_price` are nullable.
//	@Tags			product
//	@Accept			json
//	@Produce		json
//	@Param			body	body		CreateProductRequest	true	"Product to create"
//	@Success		201		{object}	CreateProductResponse
//	@Failure		400		{object}	CreateProductResponse	"INVALID_REQUEST_BODY | INVALID_NAME | INVALID_PRICE | INVALID_SALE_PRICE"
//	@Failure		500		{object}	CreateProductResponse	"INTERNAL_SERVER_ERROR"
//	@Router			/product [post]
func (h *ProductHandler) Create(c *gin.Context) {
	var req CreateProductRequest
	if err := decodeJSON(c.Request.Body, &req); err != nil {
		h.writeCreateError(c, domain.NewError(domain.ErrCodeInvalidRequestBody, err.Error()))
		return
	}
	if req.Name == nil {
		h.writeCreateError(c, domain.NewError(domain.ErrCodeInvalidName, "name is required"))
		return
	}
	if req.Price == nil {
		h.writeCreateError(c, domain.NewError(domain.ErrCodeInvalidPrice, "price is required"))
		return
	}

	p, err := h.uc.Create(c.Request.Context(), usecase.CreateProductInput{
		Name:        *req.Name,
		Description: req.Description,
		Price:       *req.Price,
		SalePrice:   req.SalePrice,
	})
	if err != nil {
		h.writeCreateError(c, err)
		return
	}
	c.JSON(http.StatusCreated, CreateProductResponse{Successful: true, Data: toProductData(p)})
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
//	@Param			id		path		int					true	"Product ID"
//	@Param			body	body		PatchProductRequest	true	"Fields to update"
//	@Success		200		{object}	BaseResponse
//	@Failure		400		{object}	BaseResponse	"INVALID_PRODUCT_ID | INVALID_REQUEST_BODY | INVALID_NAME | INVALID_PRICE | INVALID_SALE_PRICE"
//	@Failure		404		{object}	BaseResponse	"PRODUCT_NOT_FOUND"
//	@Failure		500		{object}	BaseResponse	"INTERNAL_SERVER_ERROR"
//	@Router			/product/{id} [patch]
func (h *ProductHandler) Patch(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		h.writeBaseError(c, domain.NewError(domain.ErrCodeInvalidProductID, "id must be a positive integer"))
		return
	}

	var req PatchProductRequest
	if err := decodeJSON(c.Request.Body, &req); err != nil {
		h.writeBaseError(c, domain.NewError(domain.ErrCodeInvalidRequestBody, err.Error()))
		return
	}

	if _, err := h.uc.Patch(c.Request.Context(), id, req.toDomain()); err != nil {
		h.writeBaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, BaseResponse{Successful: true})
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
	c.JSON(statusOf(code), CreateProductResponse{Successful: false, ErrorCode: string(code), Data: nil})
}

func (h *ProductHandler) writeBaseError(c *gin.Context, err error) {
	code := h.logAndCode(c, err)
	c.JSON(statusOf(code), BaseResponse{Successful: false, ErrorCode: string(code)})
}

func (h *ProductHandler) logAndCode(c *gin.Context, err error) domain.ErrorCode {
	code := domain.CodeOf(err)
	if code == domain.ErrCodeInternal {
		h.logger.ErrorContext(c.Request.Context(), "request failed", "path", c.FullPath(), "error", err)
	}
	return code
}

func statusOf(code domain.ErrorCode) int {
	switch code {
	case domain.ErrCodeProductNotFound:
		return http.StatusNotFound
	case domain.ErrCodeInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusBadRequest
	}
}
