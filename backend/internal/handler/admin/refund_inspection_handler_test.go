package admin

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func TestRefundInspectionSubscriptionQuoteHandlesMissingSubscriptions(t *testing.T) {
	handler := &RefundInspectionHandler{}
	groupID := int64(10)
	redeem := &service.RedeemCode{
		Code:    "SUB-001",
		Type:    service.RedeemTypeSubscription,
		Value:   100,
		GroupID: &groupID,
	}

	payload := handler.subscriptionQuote(redeem, 100, gin.H{"code": redeem.Code}, gin.H{})
	if payload["eligible"] != false {
		t.Fatalf("eligible want false got %#v", payload["eligible"])
	}
	if payload["reason"] != "subscription_not_found" {
		t.Fatalf("reason want subscription_not_found got %#v", payload["reason"])
	}
}

func TestRefundInspectionSubscriptionQuoteUsesRemainingRatio(t *testing.T) {
	handler := &RefundInspectionHandler{}
	groupID := int64(10)
	redeem := &service.RedeemCode{
		Code:    "SUB-001",
		Type:    service.RedeemTypeSubscription,
		Value:   120,
		GroupID: &groupID,
	}
	progress := &service.SubscriptionProgress{
		Monthly: &service.UsageWindowProgress{
			LimitUSD:     100,
			RemainingUSD: 25,
		},
	}
	payload := handler.subscriptionQuote(redeem, 80, gin.H{"code": redeem.Code}, gin.H{
		"subscriptions": []gin.H{{
			"id":       int64(9),
			"group_id": groupID,
			"progress": progress,
		}},
	})

	if payload["eligible"] != true {
		t.Fatalf("eligible want true got %#v", payload["eligible"])
	}
	if payload["suggested_amount"] != 20.0 {
		t.Fatalf("suggested amount want 20 got %#v", payload["suggested_amount"])
	}
}

func TestRefundInspectionSubscriptionQuoteAcceptsGenericMapPayload(t *testing.T) {
	handler := &RefundInspectionHandler{}
	groupID := int64(10)
	redeem := &service.RedeemCode{
		Code:    "SUB-001",
		Type:    service.RedeemTypeSubscription,
		Value:   120,
		GroupID: &groupID,
	}
	progress := &service.SubscriptionProgress{
		Monthly: &service.UsageWindowProgress{
			LimitUSD:     100,
			RemainingUSD: 50,
		},
	}
	payload := handler.subscriptionQuote(redeem, 80, gin.H{"code": redeem.Code}, gin.H{
		"subscriptions": []any{
			map[string]any{
				"id":       int64(9),
				"group_id": groupID,
				"progress": progress,
			},
		},
	})

	if payload["eligible"] != true {
		t.Fatalf("eligible want true got %#v", payload["eligible"])
	}
	if payload["suggested_amount"] != 40.0 {
		t.Fatalf("suggested amount want 40 got %#v", payload["suggested_amount"])
	}
}

func TestRefundInspectionBalanceQuoteWindowExpired(t *testing.T) {
	handler := &RefundInspectionHandler{}
	usedBy := int64(7)
	usedAt := time.Now().Add(-25 * time.Hour)
	redeem := &service.RedeemCode{
		Code:   "BAL-001",
		Type:   service.RedeemTypeBalance,
		Value:  50,
		UsedBy: &usedBy,
		UsedAt: &usedAt,
	}
	payload, err := handler.balanceQuote(nil, redeem, gin.H{"code": redeem.Code}, gin.H{})
	if err != nil {
		t.Fatalf("balance quote failed: %v", err)
	}
	if payload["eligible"] != false {
		t.Fatalf("eligible want false got %#v", payload["eligible"])
	}
	if payload["reason"] != "balance_refund_window_expired" {
		t.Fatalf("reason want balance_refund_window_expired got %#v", payload["reason"])
	}
}
