package admin

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

const refundBalanceWindow = 24 * time.Hour

// RefundInspectionHandler exposes read-only data used by external refund workflows.
type RefundInspectionHandler struct {
	redeemService       *service.RedeemService
	userService         *service.UserService
	subscriptionService *service.SubscriptionService
	usageService        *service.UsageService
}

func NewRefundInspectionHandler(
	redeemService *service.RedeemService,
	userService *service.UserService,
	subscriptionService *service.SubscriptionService,
	usageService *service.UsageService,
) *RefundInspectionHandler {
	return &RefundInspectionHandler{
		redeemService:       redeemService,
		userService:         userService,
		subscriptionService: subscriptionService,
		usageService:        usageService,
	}
}

type refundInspectionQuoteRequest struct {
	Code        string  `json:"code" binding:"required"`
	OrderAmount float64 `json:"order_amount"`
}

func (h *RefundInspectionHandler) GetRedeemCode(c *gin.Context) {
	code := strings.TrimSpace(c.Param("code"))
	if code == "" {
		response.BadRequest(c, "redeem code is required")
		return
	}
	redeem, err := h.redeemService.GetByCode(c.Request.Context(), code)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, h.redeemCodePayload(redeem))
}

func (h *RefundInspectionHandler) GetUser(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "invalid user id")
		return
	}
	payload, err := h.userPayload(c, userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, payload)
}

func (h *RefundInspectionHandler) Quote(c *gin.Context) {
	var req refundInspectionQuoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	redeem, err := h.redeemService.GetByCode(c.Request.Context(), strings.TrimSpace(req.Code))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	payload, err := h.buildQuote(c, redeem, req.OrderAmount)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, payload)
}

func (h *RefundInspectionHandler) redeemCodePayload(redeem *service.RedeemCode) gin.H {
	if redeem == nil {
		return gin.H{}
	}
	return gin.H{
		"id":            redeem.ID,
		"code":          redeem.Code,
		"type":          redeem.Type,
		"value":         roundMoney(redeem.Value),
		"status":        redeem.Status,
		"used_by":       redeem.UsedBy,
		"used_at":       redeem.UsedAt,
		"group_id":      redeem.GroupID,
		"validity_days": redeem.ValidityDays,
		"group":         groupPayload(redeem.Group),
		"user":          userSummaryPayload(redeem.User),
		"created_at":    redeem.CreatedAt,
	}
}

func (h *RefundInspectionHandler) userPayload(c *gin.Context, userID int64) (gin.H, error) {
	ctx := c.Request.Context()
	user, err := h.userService.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	subs, err := h.subscriptionService.ListUserSubscriptions(ctx, userID)
	if err != nil {
		return nil, err
	}
	subPayloads := make([]gin.H, 0, len(subs))
	for i := range subs {
		sub := subs[i]
		progress, _ := h.subscriptionService.GetSubscriptionProgress(ctx, sub.ID)
		subPayloads = append(subPayloads, gin.H{
			"id":           sub.ID,
			"user_id":      sub.UserID,
			"group_id":     sub.GroupID,
			"status":       sub.Status,
			"starts_at":    sub.StartsAt,
			"expires_at":   sub.ExpiresAt,
			"created_at":   sub.CreatedAt,
			"updated_at":   sub.UpdatedAt,
			"group":        groupPayload(sub.Group),
			"progress":     progress,
			"monthly_used": roundMoney(sub.MonthlyUsageUSD),
		})
	}
	now := time.Now()
	oneDayAgo := now.Add(-refundBalanceWindow)
	usage24h, err := h.usageService.GetStatsByUser(ctx, userID, oneDayAgo, now)
	if err != nil {
		return nil, err
	}
	return gin.H{
		"user": gin.H{
			"id":       user.ID,
			"email":    user.Email,
			"username": user.Username,
			"balance":  roundMoney(user.Balance),
			"status":   user.Status,
		},
		"subscriptions": subsPayloadOrEmpty(subPayloads),
		"usage": gin.H{
			"last_24h_actual_cost": roundMoney(usage24h.TotalActualCost),
			"last_24h":             usage24h,
		},
	}, nil
}

func (h *RefundInspectionHandler) buildQuote(c *gin.Context, redeem *service.RedeemCode, orderAmount float64) (gin.H, error) {
	base := h.redeemCodePayload(redeem)
	if redeem.UsedBy == nil {
		return gin.H{
			"redeem_code":       base,
			"eligible":          false,
			"suggested_amount":  0,
			"refund_kind":       redeem.Type,
			"reason":            "redeem_code_not_used",
			"calculation_basis": gin.H{},
		}, nil
	}
	userPayload, err := h.userPayload(c, *redeem.UsedBy)
	if err != nil {
		return nil, err
	}

	switch redeem.Type {
	case service.RedeemTypeSubscription:
		return h.subscriptionQuote(redeem, orderAmount, base, userPayload), nil
	default:
		return h.balanceQuote(c, redeem, base, userPayload)
	}
}

func (h *RefundInspectionHandler) balanceQuote(c *gin.Context, redeem *service.RedeemCode, base gin.H, userPayload gin.H) (gin.H, error) {
	if redeem.UsedAt == nil {
		return gin.H{"redeem_code": base, "user_inspection": userPayload, "eligible": false, "suggested_amount": 0, "refund_kind": redeem.Type, "reason": "redeem_code_not_used"}, nil
	}
	now := time.Now()
	if now.Sub(*redeem.UsedAt) > refundBalanceWindow {
		return gin.H{
			"redeem_code":       base,
			"user_inspection":   userPayload,
			"eligible":          false,
			"suggested_amount":  0,
			"refund_kind":       redeem.Type,
			"reason":            "balance_refund_window_expired",
			"calculation_basis": gin.H{"window_hours": 24, "used_at": redeem.UsedAt},
		}, nil
	}
	usage, err := h.usageService.GetStatsByUser(c.Request.Context(), *redeem.UsedBy, *redeem.UsedAt, now)
	if err != nil {
		return nil, err
	}
	amount := math.Max(0, redeem.Value-usage.TotalActualCost)
	return gin.H{
		"redeem_code":      base,
		"user_inspection":  userPayload,
		"eligible":         amount > 0,
		"suggested_amount": roundMoney(amount),
		"refund_kind":      redeem.Type,
		"reason":           "balance_remaining_after_usage",
		"calculation_basis": gin.H{
			"redeem_value":       roundMoney(redeem.Value),
			"actual_cost_used":   roundMoney(usage.TotalActualCost),
			"usage_window_start": redeem.UsedAt,
			"usage_window_end":   now,
		},
	}, nil
}

func (h *RefundInspectionHandler) subscriptionQuote(redeem *service.RedeemCode, orderAmount float64, base gin.H, userPayload gin.H) gin.H {
	var matched gin.H
	for _, raw := range subscriptionPayloads(userPayload["subscriptions"]) {
		if redeem.GroupID != nil && int64FromAny(raw["group_id"]) == *redeem.GroupID {
			matched = raw
			break
		}
	}
	if matched == nil {
		return gin.H{"redeem_code": base, "user_inspection": userPayload, "eligible": false, "suggested_amount": 0, "refund_kind": redeem.Type, "reason": "subscription_not_found"}
	}
	progress, _ := matched["progress"].(*service.SubscriptionProgress)
	if progress == nil || progress.Monthly == nil || progress.Monthly.LimitUSD <= 0 {
		return gin.H{"redeem_code": base, "user_inspection": userPayload, "eligible": false, "suggested_amount": 0, "refund_kind": redeem.Type, "reason": "subscription_monthly_limit_unavailable"}
	}
	paid := orderAmount
	if paid <= 0 {
		paid = redeem.Value
	}
	ratio := progress.Monthly.RemainingUSD / progress.Monthly.LimitUSD
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	amount := paid * ratio
	return gin.H{
		"redeem_code":      base,
		"user_inspection":  userPayload,
		"eligible":         amount > 0,
		"suggested_amount": roundMoney(amount),
		"refund_kind":      redeem.Type,
		"reason":           "subscription_monthly_remaining_ratio",
		"calculation_basis": gin.H{
			"paid_amount":           roundMoney(paid),
			"monthly_limit_usd":     roundMoney(progress.Monthly.LimitUSD),
			"monthly_remaining_usd": roundMoney(progress.Monthly.RemainingUSD),
			"remaining_ratio":       math.Round(ratio*10000) / 10000,
			"subscription_id":       matched["id"],
		},
	}
}

func groupPayload(group *service.Group) gin.H {
	if group == nil {
		return nil
	}
	return gin.H{
		"id":                    group.ID,
		"name":                  group.Name,
		"platform":              group.Platform,
		"subscription_type":     group.SubscriptionType,
		"daily_limit_usd":       group.DailyLimitUSD,
		"weekly_limit_usd":      group.WeeklyLimitUSD,
		"monthly_limit_usd":     group.MonthlyLimitUSD,
		"default_validity_days": group.DefaultValidityDays,
	}
}

func userSummaryPayload(user *service.User) gin.H {
	if user == nil {
		return nil
	}
	return gin.H{"id": user.ID, "email": user.Email, "username": user.Username, "status": user.Status}
}

func subsPayloadOrEmpty(items []gin.H) []gin.H {
	if items == nil {
		return []gin.H{}
	}
	return items
}

func subscriptionPayloads(v any) []gin.H {
	switch typed := v.(type) {
	case []gin.H:
		return typed
	case []map[string]any:
		out := make([]gin.H, 0, len(typed))
		for _, item := range typed {
			out = append(out, gin.H(item))
		}
		return out
	case []any:
		out := make([]gin.H, 0, len(typed))
		for _, item := range typed {
			switch m := item.(type) {
			case gin.H:
				out = append(out, m)
			case map[string]any:
				out = append(out, gin.H(m))
			}
		}
		return out
	default:
		return nil
	}
}

func int64FromAny(v any) int64 {
	switch typed := v.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	default:
		return 0
	}
}

func roundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}
