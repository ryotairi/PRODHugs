package user

import (
	"context"
	"go-service-template/internal/errorz"
	"go-service-template/internal/transport/http/middleware"
	v1 "go-service-template/internal/transport/http/v1"

	"github.com/google/uuid"
)

func (h *UserHandler) GetSudokuCaptcha(ctx context.Context, req v1.GetSudokuCaptchaRequestObject) (v1.GetSudokuCaptchaResponseObject, error) {
	userID := ctx.Value(middleware.UserIDContextKey).(uuid.UUID)

	captchaID, puzzle, err := h.svc.GenerateSudokuCaptcha(ctx, userID)
	if err != nil {
		return nil, err
	}

	return v1.GetSudokuCaptcha200JSONResponse{
		Id:     captchaID,
		Puzzle: puzzle,
	}, nil
}

func (h *UserHandler) VerifySudokuCell(ctx context.Context, req v1.VerifySudokuCellRequestObject) (v1.VerifySudokuCellResponseObject, error) {
	userID := ctx.Value(middleware.UserIDContextKey).(uuid.UUID)

	res, err := h.svc.VerifySudokuCell(ctx, req.Id, userID, req.Body.Row, req.Body.Col, req.Body.Value)
	if err != nil {
		if err == errorz.ErrCaptchaNotFound {
			return v1.VerifySudokuCell404JSONResponse{}, nil
		}
		if err == errorz.ErrCaptchaForbidden {
			return v1.VerifySudokuCell403JSONResponse{}, nil
		}
		if err == errorz.ErrCaptchaGone {
			return v1.VerifySudokuCell410JSONResponse{}, nil
		}
		return nil, err
	}

	return v1.VerifySudokuCell200JSONResponse{
		Correct: res.Correct,
		Errors:  res.Errors,
		Failed:  &res.Failed,
	}, nil
}

func (h *UserHandler) CompleteSudoku(ctx context.Context, req v1.CompleteSudokuRequestObject) (v1.CompleteSudokuResponseObject, error) {
	userID := ctx.Value(middleware.UserIDContextKey).(uuid.UUID)

	token, err := h.svc.CompleteSudoku(ctx, req.Id, userID)
	if err != nil {
		if err == errorz.ErrCaptchaNotFound {
			return v1.CompleteSudoku404JSONResponse{}, nil
		}
		if err == errorz.ErrCaptchaForbidden {
			return v1.CompleteSudoku403JSONResponse{}, nil
		}
		return nil, err
	}

	return v1.CompleteSudoku200JSONResponse{
		CaptchaToken: token,
	}, nil
}

func (h *UserHandler) GetCasinoCaptcha(ctx context.Context, req v1.GetCasinoCaptchaRequestObject) (v1.GetCasinoCaptchaResponseObject, error) {
	userID := ctx.Value(middleware.UserIDContextKey).(uuid.UUID)

	captchaID, expiresAt, err := h.svc.GenerateCasinoCaptcha(ctx, userID)
	if err != nil {
		return nil, err
	}

	return v1.GetCasinoCaptcha200JSONResponse{
		Id:        captchaID,
		ExpiresAt: expiresAt,
	}, nil
}

func (h *UserHandler) SpinCasino(ctx context.Context, req v1.SpinCasinoRequestObject) (v1.SpinCasinoResponseObject, error) {
	userID := ctx.Value(middleware.UserIDContextKey).(uuid.UUID)

	res, err := h.svc.SpinCasino(ctx, req.Id, userID)
	if err != nil {
		if err == errorz.ErrCaptchaNotFound {
			return v1.SpinCasino404JSONResponse{}, nil
		}
		if err == errorz.ErrCaptchaForbidden {
			return v1.SpinCasino403JSONResponse{}, nil
		}
		if err == errorz.ErrCaptchaGone {
			return v1.SpinCasino410JSONResponse{}, nil
		}
		return nil, err
	}

	return v1.SpinCasino200JSONResponse{
		Win:           res.Win,
		CaptchaToken:  &res.CaptchaToken,
		CooldownUntil: res.CooldownUntil,
	}, nil
}
