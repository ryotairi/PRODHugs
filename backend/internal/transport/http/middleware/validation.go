package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"go-service-template/internal/jwt"
	v1 "go-service-template/internal/transport/http/v1"
	v2 "go-service-template/internal/transport/http/v2"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/labstack/echo/v4"

	oapimiddleware "github.com/oapi-codegen/echo-middleware"
)

// OpenAPIValidationMiddleware wires the v1 OpenAPI request validator.
func OpenAPIValidationMiddleware(jwtManager *jwt.Manager) (echo.MiddlewareFunc, error) {
	swagger, err := v1.GetSwagger()
	if err != nil {
		return nil, fmt.Errorf("get v1 swagger failed: %w", err)
	}
	return openAPIValidationFor(swagger, jwtManager), nil
}

// OpenAPIV2ValidationMiddleware wires the v2 OpenAPI request validator. The
// v2 spec is mounted under a separate Echo group, so request paths are
// already stripped of the `/api/v2` prefix by the time the validator runs
// against them.
func OpenAPIV2ValidationMiddleware(jwtManager *jwt.Manager) (echo.MiddlewareFunc, error) {
	swagger, err := v2.GetSwagger()
	if err != nil {
		return nil, fmt.Errorf("get v2 swagger failed: %w", err)
	}
	return openAPIValidationFor(swagger, jwtManager), nil
}

func openAPIValidationFor(swagger *openapi3.T, jwtManager *jwt.Manager) echo.MiddlewareFunc {
	validatorOptions := &oapimiddleware.Options{
		Options: openapi3filter.Options{
			AuthenticationFunc: NewAuthenticator(jwtManager),
		},
		ErrorHandler: func(c echo.Context, err *echo.HTTPError) error {
			finalCode := err.Code
			msg := fmt.Sprintf("%v", err.Message)
			// All security/auth failures come as 403 from the OAPI middleware,
			// but the frontend expects 401 for any token issue (missing,
			// expired, invalid).
			if err.Code == http.StatusForbidden && (strings.Contains(msg, "Authorization") ||
				strings.Contains(msg, "token") ||
				strings.Contains(msg, "security")) {
				finalCode = http.StatusUnauthorized
			}
			return c.JSON(finalCode, map[string]interface{}{
				"type":   "validation_error",
				"title":  "Request validation failed",
				"status": finalCode,
				"detail": err.Message,
			})
		},
	}

	return oapimiddleware.OapiRequestValidatorWithOptions(swagger, validatorOptions)
}
