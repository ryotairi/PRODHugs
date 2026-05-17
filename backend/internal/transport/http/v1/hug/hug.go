package hug

import (
	"context"
	"errors"
	"time"

	"go-service-template/internal/errorz"
	"go-service-template/internal/transport/http/middleware"
	v1 "go-service-template/internal/transport/http/v1"

	"github.com/google/uuid"
)

func (h *HugHandler) SuggestHug(ctx context.Context, req v1.SuggestHugRequestObject) (v1.SuggestHugResponseObject, error) {
	giverID := ctx.Value(middleware.UserIDContextKey).(uuid.UUID)
	receiverID := req.UserId

	if giverID == receiverID {
		return v1.SuggestHug400JSONResponse{
			BadRequestJSONResponse: v1.BadRequestJSONResponse{
				Code:    v1.CANNOTHUGSELF,
				Message: "You cannot hug yourself",
			},
		}, nil
	}

	hugType := "standard"
	if req.Body != nil && req.Body.HugType != nil {
		hugType = string(*req.Body.HugType)
	}

	var comment *string
	if req.Body != nil && req.Body.Comment != nil && *req.Body.Comment != "" {
		c := *req.Body.Comment
		if len([]rune(c)) > 140 {
			return v1.SuggestHug400JSONResponse{
				BadRequestJSONResponse: v1.BadRequestJSONResponse{
					Code:    v1.COMMENTTOOLONG,
					Message: "Comment must be at most 140 characters",
				},
			}, nil
		}
		comment = &c
	}

	var captchaToken *string
	if req.Body != nil && req.Body.CaptchaToken != nil && *req.Body.CaptchaToken != "" {
		captchaToken = req.Body.CaptchaToken
	}

	hugResult, receiver, err := h.svc.SuggestHug(ctx, giverID, receiverID, hugType, comment, captchaToken)
	if err != nil {
		if errors.Is(err, errorz.ErrHugTypeLocked) {
			return v1.SuggestHug409JSONResponse{
				ConflictJSONResponse: v1.ConflictJSONResponse{Code: v1.HUGTYPELOCKED, Message: "Hug type not unlocked for this pair's intimacy level"},
			}, nil
		}
		if errors.Is(err, errorz.ErrAlreadyHasPendingHug) {
			return v1.SuggestHug409JSONResponse{
				ConflictJSONResponse: v1.ConflictJSONResponse{Code: v1.ALREADYHASPENDINGHUG, Message: "You already have a pending outgoing hug"},
			}, nil
		}
		if errors.Is(err, errorz.ErrPendingHugExists) {
			return v1.SuggestHug409JSONResponse{
				ConflictJSONResponse: v1.ConflictJSONResponse{Code: v1.PENDINGHUGEXISTS, Message: "Pending hug already exists for this pair"},
			}, nil
		}
		if errors.Is(err, errorz.ErrReversePendingHugExists) {
			return v1.SuggestHug409JSONResponse{
				ConflictJSONResponse: v1.ConflictJSONResponse{Code: v1.PENDINGHUGEXISTS, Message: "This user has already suggested a hug to you"},
			}, nil
		}
		if errors.Is(err, errorz.ErrDeclineCooldownActive) {
			return v1.SuggestHug429JSONResponse{TooManyRequestsJSONResponse: v1.TooManyRequestsJSONResponse{Code: v1.DECLINECOOLDOWNACTIVE, Message: "Decline cooldown active"}}, nil
		}
		if errors.Is(err, errorz.ErrHugCooldownActive) {
			return v1.SuggestHug429JSONResponse{TooManyRequestsJSONResponse: v1.TooManyRequestsJSONResponse{Code: v1.COOLDOWNACTIVE, Message: "You need to wait before hugging this user again"}}, nil
		}
		if errors.Is(err, errorz.ErrUserBlocked) {
			return v1.SuggestHug409JSONResponse{ConflictJSONResponse: v1.ConflictJSONResponse{Code: v1.USERBLOCKED, Message: "User is blocked"}}, nil
		}
		if errors.Is(err, errorz.ErrUserNotFound) {
			return v1.SuggestHug404JSONResponse{NotFoundJSONResponse: v1.NotFoundJSONResponse{Code: v1.USERNOTFOUND, Message: "User not found"}}, nil
		}
		if err == errorz.ErrCaptchaRequired {
			return v1.SuggestHug400JSONResponse{
				BadRequestJSONResponse: v1.BadRequestJSONResponse{
					Code:    v1.CAPTCHAREQUIRED,
					Message: "Captcha required before hugging",
				},
			}, nil
		}
		if err == errorz.ErrCaptchaFailed {
			return v1.SuggestHug400JSONResponse{
				BadRequestJSONResponse: v1.BadRequestJSONResponse{
					Code:    v1.CAPTCHAFAILED,
					Message: "Invalid or expired captcha token",
				},
			}, nil
		}
		return nil, err
	}

	ht := v1.HugType(hugResult.HugType)
	resp := v1.SuggestHug201JSONResponse{
		Id:         hugResult.ID,
		GiverId:    hugResult.GiverID,
		ReceiverId: hugResult.ReceiverID,
		CreatedAt:  hugResult.CreatedAt,
		Status:     v1.HugStatus(hugResult.Status),
		HugType:    ht,
		AcceptedAt: hugResult.AcceptedAt,
		Comment:    hugResult.Comment,
	}
	if receiver != nil {
		resp.ReceiverUsername = &receiver.Username
		if receiver.Gender != nil {
			g := v1.Gender(*receiver.Gender)
			resp.ReceiverGender = &g
		}
	}
	return resp, nil
}

func (h *HugHandler) GetHugDetail(ctx context.Context, req v1.GetHugDetailRequestObject) (v1.GetHugDetailResponseObject, error) {
	requesterID := ctx.Value(middleware.UserIDContextKey).(uuid.UUID)
	role, _ := ctx.Value(middleware.UserRoleContextKey).(string)
	isAdmin := role == "admin"

	detail, err := h.svc.GetHugDetail(ctx, req.HugId, requesterID, isAdmin)
	if err != nil {
		if errors.Is(err, errorz.ErrHugNotFound) {
			return v1.GetHugDetail404JSONResponse{NotFoundJSONResponse: v1.NotFoundJSONResponse{Code: v1.HUGNOTFOUND, Message: "Hug not found"}}, nil
		}
		return nil, err
	}

	ht := v1.HugType(detail.HugType)
	resp := v1.GetHugDetail200JSONResponse{
		Id:                  detail.ID,
		GiverId:             detail.GiverID,
		ReceiverId:          detail.ReceiverID,
		GiverUsername:       detail.GiverUsername,
		ReceiverUsername:    detail.ReceiverUsername,
		GiverDisplayName:    detail.GiverDisplayName,
		ReceiverDisplayName: detail.ReceiverDisplayName,
		Status:              v1.HugDetailStatus(detail.Status),
		HugType:             ht,
		Comment:             detail.Comment,
		StreakTier:          &detail.StreakTier,
		CreatedAt:           detail.CreatedAt,
		AcceptedAt:          detail.AcceptedAt,
	}
	if detail.GiverGender != nil {
		g := v1.Gender(*detail.GiverGender)
		resp.GiverGender = &g
	}

	return resp, nil
}

func (h *HugHandler) AcceptHug(ctx context.Context, req v1.AcceptHugRequestObject) (v1.AcceptHugResponseObject, error) {
	receiverID := ctx.Value(middleware.UserIDContextKey).(uuid.UUID)
	hugID := req.HugId

	hug, err := h.svc.AcceptHug(ctx, hugID, receiverID)
	if err != nil {
		if errors.Is(err, errorz.ErrHugNotFound) {
			return v1.AcceptHug404JSONResponse{NotFoundJSONResponse: v1.NotFoundJSONResponse{Code: v1.HUGNOTFOUND, Message: "Hug not found"}}, nil
		}
		if errors.Is(err, errorz.ErrHugNotPending) {
			return v1.AcceptHug409JSONResponse{ConflictJSONResponse: v1.ConflictJSONResponse{Code: v1.HUGNOTPENDING, Message: "Hug is not pending"}}, nil
		}
		if errors.Is(err, errorz.ErrHugExpired) {
			return v1.AcceptHug410JSONResponse{GoneJSONResponse: v1.GoneJSONResponse{Code: v1.HUGEXPIRED, Message: "Hug suggestion expired"}}, nil
		}
		return nil, err
	}

	ht := v1.HugType(hug.HugType)
	return v1.AcceptHug200JSONResponse{Id: hug.ID, GiverId: hug.GiverID, ReceiverId: hug.ReceiverID, CreatedAt: hug.CreatedAt, Status: v1.HugStatus(hug.Status), HugType: ht, AcceptedAt: hug.AcceptedAt}, nil
}

func (h *HugHandler) DeclineHug(ctx context.Context, req v1.DeclineHugRequestObject) (v1.DeclineHugResponseObject, error) {
	receiverID := ctx.Value(middleware.UserIDContextKey).(uuid.UUID)
	hugID := req.HugId

	err := h.svc.DeclineHug(ctx, hugID, receiverID)
	if err != nil {
		if errors.Is(err, errorz.ErrHugNotFound) {
			return v1.DeclineHug404JSONResponse{NotFoundJSONResponse: v1.NotFoundJSONResponse{Code: v1.HUGNOTFOUND, Message: "Hug not found"}}, nil
		}
		if errors.Is(err, errorz.ErrHugNotPending) {
			return v1.DeclineHug409JSONResponse{ConflictJSONResponse: v1.ConflictJSONResponse{Code: v1.HUGNOTPENDING, Message: "Hug is not pending"}}, nil
		}
		if errors.Is(err, errorz.ErrHugExpired) {
			return v1.DeclineHug410JSONResponse{GoneJSONResponse: v1.GoneJSONResponse{Code: v1.HUGEXPIRED, Message: "Hug suggestion expired"}}, nil
		}
		return nil, err
	}

	return v1.DeclineHug200JSONResponse{Message: "Hug declined"}, nil
}

func (h *HugHandler) CancelHug(ctx context.Context, req v1.CancelHugRequestObject) (v1.CancelHugResponseObject, error) {
	giverID := ctx.Value(middleware.UserIDContextKey).(uuid.UUID)
	hugID := req.HugId

	err := h.svc.CancelHug(ctx, hugID, giverID)
	if err != nil {
		if errors.Is(err, errorz.ErrHugNotFound) {
			return v1.CancelHug404JSONResponse{NotFoundJSONResponse: v1.NotFoundJSONResponse{Code: v1.HUGNOTFOUND, Message: "Hug not found"}}, nil
		}
		if errors.Is(err, errorz.ErrHugNotPending) {
			return v1.CancelHug409JSONResponse{ConflictJSONResponse: v1.ConflictJSONResponse{Code: v1.HUGNOTPENDING, Message: "Hug is not pending"}}, nil
		}
		if errors.Is(err, errorz.ErrHugExpired) {
			return v1.CancelHug410JSONResponse{GoneJSONResponse: v1.GoneJSONResponse{Code: v1.HUGEXPIRED, Message: "Hug suggestion expired"}}, nil
		}
		return nil, err
	}

	return v1.CancelHug200JSONResponse{Message: "Hug cancelled"}, nil
}

func (h *HugHandler) GetHugInbox(ctx context.Context, req v1.GetHugInboxRequestObject) (v1.GetHugInboxResponseObject, error) {
	userID := ctx.Value(middleware.UserIDContextKey).(uuid.UUID)

	hugs, err := h.svc.GetPendingInbox(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make(v1.GetHugInbox200JSONResponse, len(hugs))
	for i, hg := range hugs {
		ht := v1.HugType(hg.HugType)
		item := v1.PendingHugInboxItem{
			Id:               hg.ID,
			GiverId:          hg.GiverID,
			ReceiverId:       hg.ReceiverID,
			GiverUsername:    hg.GiverUsername,
			GiverDisplayName: hg.GiverDisplayName,
			HugType:          ht,
			Comment:          hg.Comment,
			CreatedAt:        hg.CreatedAt,
		}
		if hg.GiverGender != nil {
			g := v1.Gender(*hg.GiverGender)
			item.GiverGender = &g
		}
		result[i] = item
	}

	return result, nil
}

func (h *HugHandler) GetHugInboxCount(ctx context.Context, req v1.GetHugInboxCountRequestObject) (v1.GetHugInboxCountResponseObject, error) {
	userID := ctx.Value(middleware.UserIDContextKey).(uuid.UUID)

	count, err := h.svc.GetInboxCount(ctx, userID)
	if err != nil {
		return nil, err
	}

	return v1.GetHugInboxCount200JSONResponse{Count: int(count)}, nil
}

func (h *HugHandler) GetOutgoingHugs(ctx context.Context, req v1.GetOutgoingHugsRequestObject) (v1.GetOutgoingHugsResponseObject, error) {
	userID := ctx.Value(middleware.UserIDContextKey).(uuid.UUID)

	hugs, slotInfo, err := h.svc.GetOutgoingHugs(ctx, userID)
	if err != nil {
		return nil, err
	}

	items := make([]v1.OutgoingPendingHug, len(hugs))
	for i, hug := range hugs {
		ht := v1.HugType(hug.HugType)
		item := v1.OutgoingPendingHug{
			Id:                  hug.ID,
			GiverId:             hug.GiverID,
			ReceiverId:          hug.ReceiverID,
			ReceiverUsername:    hug.ReceiverUsername,
			ReceiverDisplayName: hug.ReceiverDisplayName,
			HugType:             ht,
			Comment:             hug.Comment,
			CreatedAt:           hug.CreatedAt,
		}
		if hug.ReceiverGender != nil {
			g := v1.Gender(*hug.ReceiverGender)
			item.ReceiverGender = &g
		}
		items[i] = item
	}

	slots := v1.HugSlotInfo{
		TotalSlots: int(slotInfo.TotalSlots),
		UsedSlots:  int(slotInfo.UsedSlots),
	}
	if slotInfo.NextSlotCost != nil {
		cost := int(*slotInfo.NextSlotCost)
		slots.NextSlotCost = &cost
	}

	return v1.GetOutgoingHugs200JSONResponse{
		Hugs:  items,
		Slots: slots,
	}, nil
}

func (h *HugHandler) BuyHugSlot(ctx context.Context, req v1.BuyHugSlotRequestObject) (v1.BuyHugSlotResponseObject, error) {
	userID := ctx.Value(middleware.UserIDContextKey).(uuid.UUID)

	slotInfo, newBalance, err := h.svc.BuyHugSlot(ctx, userID)
	if err != nil {
		if errors.Is(err, errorz.ErrMaxSlotsReached) {
			return v1.BuyHugSlot409JSONResponse{
				ConflictJSONResponse: v1.ConflictJSONResponse{Code: v1.MAXSLOTSREACHED, Message: "Maximum hug slots reached"},
			}, nil
		}
		if errors.Is(err, errorz.ErrInsufficientBalance) {
			return v1.BuyHugSlot400JSONResponse{
				BadRequestJSONResponse: v1.BadRequestJSONResponse{Code: v1.INSUFFICIENTBALANCE, Message: "Insufficient balance"},
			}, nil
		}
		return nil, err
	}

	slots := v1.HugSlotInfo{
		TotalSlots: int(slotInfo.TotalSlots),
		UsedSlots:  int(slotInfo.UsedSlots),
	}
	if slotInfo.NextSlotCost != nil {
		cost := int(*slotInfo.NextSlotCost)
		slots.NextSlotCost = &cost
	}

	return v1.BuyHugSlot200JSONResponse{
		Slots:      slots,
		NewBalance: int(newBalance),
	}, nil
}

func (h *HugHandler) GetCooldown(ctx context.Context, req v1.GetCooldownRequestObject) (v1.GetCooldownResponseObject, error) {
	userA := ctx.Value(middleware.UserIDContextKey).(uuid.UUID)
	userB := req.UserId

	info, err := h.svc.GetCooldownInfo(ctx, userA, userB)
	if err != nil {
		return nil, err
	}

	resp := v1.GetCooldown200JSONResponse{
		UserAId:                  info.Cooldown.UserAID,
		UserBId:                  info.Cooldown.UserBID,
		CooldownSeconds:          int(info.Cooldown.CooldownSeconds),
		RemainingSeconds:         int(info.RemainingSeconds),
		CanHug:                   info.CanHug,
		EffectiveCooldownSeconds: int(info.EffectiveCooldown),
		IntimacyReductionPct:     info.IntimacyReductionPct,
	}
	if info.DeclineRemaining > 0 {
		dr := int(info.DeclineRemaining)
		resp.DeclineCooldownRemaining = &dr
		if dr > int(info.RemainingSeconds) {
			resp.RemainingSeconds = dr
		}
	}

	return resp, nil
}

func (h *HugHandler) UpgradeCooldown(ctx context.Context, req v1.UpgradeCooldownRequestObject) (v1.UpgradeCooldownResponseObject, error) {
	payerID := ctx.Value(middleware.UserIDContextKey).(uuid.UUID)
	otherUserID := req.UserId

	cd, err := h.svc.UpgradeCooldown(ctx, payerID, otherUserID)
	if err != nil {
		if errors.Is(err, errorz.ErrInsufficientBalance) {
			return v1.UpgradeCooldown400JSONResponse{
				BadRequestJSONResponse: v1.BadRequestJSONResponse{
					Code:    v1.INSUFFICIENTBALANCE,
					Message: "Not enough balance to upgrade cooldown",
				},
			}, nil
		}
		return nil, err
	}

	// Calculate remaining time
	elapsed := time.Since(cd.LastHugAt)
	remaining := time.Duration(cd.CooldownSeconds)*time.Second - elapsed
	if remaining < 0 {
		remaining = 0
	}

	return v1.UpgradeCooldown200JSONResponse{
		UserAId:          cd.UserAID,
		UserBId:          cd.UserBID,
		CooldownSeconds:  int(cd.CooldownSeconds),
		RemainingSeconds: int(remaining.Seconds()),
		CanHug:           remaining <= 0,
	}, nil
}
