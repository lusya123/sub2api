package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

func TestThirdPaySignMatchesDujiaoNextVector(t *testing.T) {
	t.Parallel()

	params := map[string]string{
		"mch_id":       "5000",
		"type":         "1",
		"amount":       "1",
		"out_trade_no": "test_0123456789",
		"notify_url":   "https://www.baidu.com",
		"subject":      "test",
		"ip":           "127.0.0.1",
		"body":         "",
		"sign_type":    "MD5",
		"sign":         "ignored",
	}
	got := thirdPaySign(params, "xxxxxxxx")
	const want = "584a56b8e3a2634b3b8009f362e59b46"
	if got != want {
		t.Fatalf("thirdPaySign() = %s, want %s", got, want)
	}
	if !thirdPayVerifySign(params, "xxxxxxxx", strings.ToUpper(want)) {
		t.Fatal("thirdPayVerifySign() should accept uppercase MD5 signatures")
	}
}

func TestThirdPayCreateAndQuery(t *testing.T) {
	t.Parallel()

	var createForm url.Values
	var queryForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("reserve"); got != "reserve-token" {
			t.Fatalf("reserve header = %q, want reserve-token", got)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		switch r.URL.Path {
		case "/ThirdApi/Pay/create_order":
			createForm = cloneValues(r.PostForm)
			writeJSON(t, w, map[string]any{
				"code": 200,
				"result": map[string]any{
					"pay_url":     "https://pay.example/checkout/1",
					"qr_code":     "qr-content",
					"platform_sn": "tp-001",
				},
			})
		case "/ThirdApi/Pay/select_order":
			queryForm = cloneValues(r.PostForm)
			writeJSON(t, w, map[string]any{
				"code": 200,
				"result": map[string]any{
					"status":       "3",
					"amount":       "12.34",
					"platform_sn":  "tp-001",
					"out_trade_no": "sub2_20260626abcd1234",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider := newTestThirdPay(t, server.URL)
	createResp, err := provider.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID:     "sub2_20260626abcd1234",
		Amount:      "12.34",
		PaymentType: payment.TypeAlipay,
		Subject:     "Sub2API recharge",
		NotifyURL:   "https://merchant.example/api/v1/payment/webhook/thirdpay",
		ClientIP:    "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("CreatePayment() error = %v", err)
	}
	if createResp.PayURL != "https://pay.example/checkout/1" || createResp.QRCode != "qr-content" || createResp.TradeNo != "tp-001" {
		t.Fatalf("CreatePayment() response = %+v", createResp)
	}
	assertFormValue(t, createForm, "mch_id", "mch-1")
	assertFormValue(t, createForm, "type", "2")
	assertFormValue(t, createForm, "out_trade_no", "sub2_20260626abcd1234")
	assertFormValue(t, createForm, "amount", "12.34")
	assertFormValue(t, createForm, "ip", "203.0.113.10")
	if !thirdPayVerifySign(singleFormValues(createForm), "secret-1", createForm.Get("sign")) {
		t.Fatal("create form signature did not verify")
	}

	queryResp, err := provider.QueryOrder(context.Background(), "sub2_20260626abcd1234")
	if err != nil {
		t.Fatalf("QueryOrder() error = %v", err)
	}
	if queryResp.Status != payment.ProviderStatusPaid || queryResp.Amount != 12.34 || queryResp.TradeNo != "tp-001" {
		t.Fatalf("QueryOrder() response = %+v", queryResp)
	}
	assertFormValue(t, queryForm, "out_trade_no", "sub2_20260626abcd1234")
	if !thirdPayVerifySign(singleFormValues(queryForm), "secret-1", queryForm.Get("sign")) {
		t.Fatal("query form signature did not verify")
	}
}

func TestThirdPayVerifyNotification(t *testing.T) {
	t.Parallel()

	provider := newTestThirdPay(t, "https://thirdpay.example")
	params := map[string]string{
		"out_trade_no": "sub2_20260626abcd1234",
		"sys_sn":       "tp-001",
		"status":       "SUCCESS",
		"amount":       "12.34",
	}
	params["sign"] = thirdPaySign(params, "secret-1")
	params["sign_type"] = thirdPayDefaultSignType

	values := url.Values{}
	for key, value := range params {
		values.Set(key, value)
	}
	notification, err := provider.VerifyNotification(context.Background(), values.Encode(), nil)
	if err != nil {
		t.Fatalf("VerifyNotification() error = %v", err)
	}
	if notification.OrderID != "sub2_20260626abcd1234" ||
		notification.TradeNo != "tp-001" ||
		notification.Status != payment.ProviderStatusSuccess ||
		notification.Amount != 12.34 {
		t.Fatalf("VerifyNotification() = %+v", notification)
	}
}

func TestThirdPayVerifyNotificationAcceptsAggregatorAliases(t *testing.T) {
	t.Parallel()

	provider := newTestThirdPay(t, "https://thirdpay.example")
	params := map[string]string{
		"order_no":    "sub2_20260626abcd1234",
		"platform_sn": "tp-001",
		"status":      "3",
		"money":       "12.34",
	}
	params["sign"] = thirdPaySign(params, "secret-1")
	params["sign_type"] = thirdPayDefaultSignType

	values := url.Values{}
	for key, value := range params {
		values.Set(key, value)
	}
	notification, err := provider.VerifyNotification(context.Background(), values.Encode(), nil)
	if err != nil {
		t.Fatalf("VerifyNotification() error = %v", err)
	}
	if notification.OrderID != "sub2_20260626abcd1234" ||
		notification.TradeNo != "tp-001" ||
		notification.Status != payment.ProviderStatusSuccess ||
		notification.Amount != 12.34 {
		t.Fatalf("VerifyNotification() = %+v", notification)
	}
}

func newTestThirdPay(t *testing.T, baseURL string) *ThirdPay {
	t.Helper()
	provider, err := NewThirdPay("thirdpay-test", map[string]string{
		"createOrderUrl": baseURL + "/ThirdApi/Pay/create_order",
		"selectOrderUrl": baseURL + "/ThirdApi/Pay/select_order",
		"merchantId":     "mch-1",
		"merchantKey":    "secret-1",
		"reserve":        "reserve-token",
		"notifyUrl":      "https://merchant.example/api/v1/payment/webhook/thirdpay",
	})
	if err != nil {
		t.Fatalf("NewThirdPay() error = %v", err)
	}
	return provider
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("Encode response: %v", err)
	}
}

func cloneValues(values url.Values) url.Values {
	cloned := make(url.Values, len(values))
	for key, value := range values {
		cloned[key] = append([]string(nil), value...)
	}
	return cloned
}

func singleFormValues(values url.Values) map[string]string {
	out := make(map[string]string, len(values))
	for key := range values {
		out[key] = values.Get(key)
	}
	return out
}

func assertFormValue(t *testing.T, values url.Values, key, want string) {
	t.Helper()
	if got := values.Get(key); got != want {
		t.Fatalf("form[%s] = %q, want %q (form=%v)", key, got, want, values)
	}
}
