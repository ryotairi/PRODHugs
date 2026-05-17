package v2

import (
	"context"
	"errors"

	"go-service-template/internal/errorz"
	"go-service-template/internal/models"

	"github.com/google/uuid"
)

// resolveNoteTarget shares the same UUID / "@username" / username resolution
// rules as the profile endpoint, exposed via the note service. Returns a 404
// response when the target doesn't exist.
func (h *Handler) resolveNoteTarget(ctx context.Context, raw string) (*models.User, error) {
	return h.noteSvc.ResolveTarget(ctx, raw)
}

// ── GET /users/{usernameOrId}/note ─────────────────────────────────────────

func (h *Handler) GetUserNoteV2(ctx context.Context, req GetUserNoteV2RequestObject) (GetUserNoteV2ResponseObject, error) {
	authorID, _ := userIDFromCtx(ctx)
	if authorID == uuid.Nil {
		return GetUserNoteV2401JSONResponse{UnauthorizedJSONResponse{Code: "UNAUTHORIZED", Message: "missing user"}}, nil
	}

	target, err := h.resolveNoteTarget(ctx, req.UsernameOrId)
	if err != nil {
		if errors.Is(err, errorz.ErrUserNotFound) {
			return GetUserNoteV2404JSONResponse{NotFoundJSONResponse{Code: "USER_NOT_FOUND", Message: "User not found"}}, nil
		}
		return nil, err
	}

	note, err := h.noteSvc.Get(ctx, authorID, target.ID)
	if err != nil {
		return nil, err
	}
	if note == nil {
		return GetUserNoteV2404JSONResponse{NotFoundJSONResponse{Code: "NOTE_NOT_FOUND", Message: "Note not found"}}, nil
	}
	return GetUserNoteV2200JSONResponse(toUserNoteDTO(note)), nil
}

// ── PUT /users/{usernameOrId}/note ─────────────────────────────────────────

func (h *Handler) UpsertUserNoteV2(ctx context.Context, req UpsertUserNoteV2RequestObject) (UpsertUserNoteV2ResponseObject, error) {
	authorID, _ := userIDFromCtx(ctx)
	if authorID == uuid.Nil {
		return UpsertUserNoteV2401JSONResponse{UnauthorizedJSONResponse{Code: "UNAUTHORIZED", Message: "missing user"}}, nil
	}
	if req.Body == nil {
		return UpsertUserNoteV2400JSONResponse{BadRequestJSONResponse{Code: "BAD_REQUEST", Message: "missing body"}}, nil
	}

	target, err := h.resolveNoteTarget(ctx, req.UsernameOrId)
	if err != nil {
		if errors.Is(err, errorz.ErrUserNotFound) {
			return UpsertUserNoteV2404JSONResponse{NotFoundJSONResponse{Code: "USER_NOT_FOUND", Message: "User not found"}}, nil
		}
		return nil, err
	}

	note, err := h.noteSvc.Upsert(ctx, authorID, target.ID, req.Body.Content)
	if err != nil {
		if errors.Is(err, errorz.ErrNoteInvalid) {
			return UpsertUserNoteV2400JSONResponse{BadRequestJSONResponse{Code: "NOTE_INVALID", Message: "Note must be 1–256 characters"}}, nil
		}
		return nil, err
	}
	return UpsertUserNoteV2200JSONResponse(toUserNoteDTO(note)), nil
}

// ── DELETE /users/{usernameOrId}/note ──────────────────────────────────────

func (h *Handler) DeleteUserNoteV2(ctx context.Context, req DeleteUserNoteV2RequestObject) (DeleteUserNoteV2ResponseObject, error) {
	authorID, _ := userIDFromCtx(ctx)
	if authorID == uuid.Nil {
		return DeleteUserNoteV2401JSONResponse{UnauthorizedJSONResponse{Code: "UNAUTHORIZED", Message: "missing user"}}, nil
	}

	target, err := h.resolveNoteTarget(ctx, req.UsernameOrId)
	if err != nil {
		if errors.Is(err, errorz.ErrUserNotFound) {
			return DeleteUserNoteV2404JSONResponse{NotFoundJSONResponse{Code: "USER_NOT_FOUND", Message: "User not found"}}, nil
		}
		return nil, err
	}

	if err := h.noteSvc.Delete(ctx, authorID, target.ID); err != nil {
		return nil, err
	}
	return DeleteUserNoteV2204Response{}, nil
}

// ── GET /notes ─────────────────────────────────────────────────────────────

func (h *Handler) ListUserNotesV2(ctx context.Context, req ListUserNotesV2RequestObject) (ListUserNotesV2ResponseObject, error) {
	authorID, _ := userIDFromCtx(ctx)
	if authorID == uuid.Nil {
		return ListUserNotesV2401JSONResponse{UnauthorizedJSONResponse{Code: "UNAUTHORIZED", Message: "missing user"}}, nil
	}

	limit := int32(50)
	offset := int32(0)
	if req.Params.Limit != nil && *req.Params.Limit > 0 {
		limit = int32(*req.Params.Limit)
	}
	if req.Params.Offset != nil && *req.Params.Offset >= 0 {
		offset = int32(*req.Params.Offset)
	}

	notes, err := h.noteSvc.List(ctx, authorID, limit, offset)
	if err != nil {
		return nil, err
	}

	out := make(ListUserNotesV2200JSONResponse, len(notes))
	for i, n := range notes {
		out[i] = toUserNoteDTO(n)
	}
	return out, nil
}

// toUserNoteDTO maps the domain note to the wire shape. target_username and
// target_display_name are only populated by the list endpoint (the per-target
// endpoints know the target out-of-band from the URL).
func toUserNoteDTO(n *models.UserNote) UserNote {
	dto := UserNote{
		TargetId:  n.TargetID,
		Content:   n.Content,
		UpdatedAt: n.UpdatedAt,
	}
	if n.TargetUsername != "" {
		u := n.TargetUsername
		dto.TargetUsername = &u
	}
	if n.TargetDisplayName != nil {
		dto.TargetDisplayName = n.TargetDisplayName
	}
	return dto
}
