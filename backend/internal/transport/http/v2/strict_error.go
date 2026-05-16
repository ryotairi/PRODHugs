package v2

import (
	"log/slog"
	"net/http"

	"go-service-template/internal/errorz"

	"github.com/labstack/echo/v4"
)

// StrictErrorMiddleware mirrors v1's middleware.StrictErrorMiddleware for the
// v2 strict server. v1 and v2 each generate their own StrictHandlerFunc type,
// so the middleware can't be shared as a single function.
func StrictErrorMiddleware(f StrictHandlerFunc, operationID string) StrictHandlerFunc {
	return func(ctx echo.Context, request interface{}) (response interface{}, err error) {
		res, err := f(ctx, request)
		if err != nil {
			slog.Error("operation failed", "operation_id", operationID, "error", err)
			return nil, echo.NewHTTPError(http.StatusInternalServerError, errorz.ErrInternalServerError.Error())
		}
		return res, nil
	}
}
