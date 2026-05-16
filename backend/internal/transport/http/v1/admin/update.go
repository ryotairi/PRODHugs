package admin

import (
	"context"
	"errors"
	"go-service-template/internal/errorz"
	v1 "go-service-template/internal/transport/http/v1"
)

func (h *AdminHandler) AdminUpdateUsername(ctx context.Context, req v1.AdminUpdateUsernameRequestObject) (v1.AdminUpdateUsernameResponseObject, error) {
	u, err := h.svc.AdminUpdateUsername(ctx, req.UserId, req.Body.Username)
	if err != nil {
		if errors.Is(err, errorz.ErrUserAlreadyExists) {
			return v1.AdminUpdateUsername409JSONResponse{
				ConflictJSONResponse: v1.ConflictJSONResponse{
					Message: "username already taken",
					Code:    v1.USERALREADYEXISTS,
				},
			}, nil
		}
		return nil, err
	}

	return v1.AdminUpdateUsername200JSONResponse(toV1AdminUser(u)), nil
}

func (h *AdminHandler) AdminUpdateGender(ctx context.Context, req v1.AdminUpdateGenderRequestObject) (v1.AdminUpdateGenderResponseObject, error) {
	var gender *string
	if req.Body.Gender != nil {
		g := string(*req.Body.Gender)
		gender = &g
	}

	u, err := h.svc.AdminUpdateGender(ctx, req.UserId, gender)
	if err != nil {
		return nil, err
	}

	return v1.AdminUpdateGender200JSONResponse(toV1AdminUser(u)), nil
}

func (h *AdminHandler) AdminUpdateDisplayName(ctx context.Context, req v1.AdminUpdateDisplayNameRequestObject) (v1.AdminUpdateDisplayNameResponseObject, error) {
	u, err := h.svc.AdminUpdateDisplayName(ctx, req.UserId, req.Body.DisplayName)
	if err != nil {
		return nil, err
	}

	return v1.AdminUpdateDisplayName200JSONResponse(toV1AdminUser(u)), nil
}

func (h *AdminHandler) AdminUpdateTag(ctx context.Context, req v1.AdminUpdateTagRequestObject) (v1.AdminUpdateTagResponseObject, error) {
	u, err := h.svc.AdminUpdateTag(ctx, req.UserId, req.Body.Tag)
	if err != nil {
		return nil, err
	}

	return v1.AdminUpdateTag200JSONResponse(toV1AdminUser(u)), nil
}

func (h *AdminHandler) AdminUpdateSpecialTag(ctx context.Context, req v1.AdminUpdateSpecialTagRequestObject) (v1.AdminUpdateSpecialTagResponseObject, error) {
	u, err := h.svc.AdminUpdateSpecialTag(ctx, req.UserId, req.Body.SpecialTag)
	if err != nil {
		return nil, err
	}

	return v1.AdminUpdateSpecialTag200JSONResponse(toV1AdminUser(u)), nil
}

func (h *AdminHandler) AdminUpdatePassword(ctx context.Context, req v1.AdminUpdatePasswordRequestObject) (v1.AdminUpdatePasswordResponseObject, error) {
	err := h.svc.AdminUpdatePassword(ctx, req.UserId, req.Body.Password)
	if err != nil {
		return nil, err
	}

	return v1.AdminUpdatePassword200JSONResponse{
		Message: "password updated",
	}, nil
}

func (h *AdminHandler) AdminUpdateCaptchaType(ctx context.Context, req v1.AdminUpdateCaptchaTypeRequestObject) (v1.AdminUpdateCaptchaTypeResponseObject, error) {
	u, err := h.svc.AdminUpdateCaptchaType(ctx, req.UserId, string(req.Body.CaptchaType))
	if err != nil {
		if errors.Is(err, errorz.ErrUserNotFound) {
			return v1.AdminUpdateCaptchaType404JSONResponse{
				NotFoundJSONResponse: v1.NotFoundJSONResponse{
					Message: "user not found",
					Code:    v1.USERNOTFOUND,
				},
			}, nil
		}
		return nil, err
	}

	return v1.AdminUpdateCaptchaType200JSONResponse(toV1AdminUser(u)), nil
}

func (h *AdminHandler) AdminClearPromotion(ctx context.Context, req v1.AdminClearPromotionRequestObject) (v1.AdminClearPromotionResponseObject, error) {
	u, err := h.svc.AdminClearPromotion(ctx, req.UserId)
	if err != nil {
		if errors.Is(err, errorz.ErrUserNotFound) {
			return v1.AdminClearPromotion404JSONResponse{
				NotFoundJSONResponse: v1.NotFoundJSONResponse{
					Message: "user not found",
					Code:    v1.USERNOTFOUND,
				},
			}, nil
		}
		return nil, err
	}

	return v1.AdminClearPromotion200JSONResponse(toV1User(u)), nil
}
