package provider

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

const (
	thirdPayDefaultSignType   = "MD5"
	thirdPayDefaultWechatType = "1"
	thirdPayDefaultAlipayType = "2"
	thirdPaySuccessCode       = 200
	thirdPayTradeSuccess      = "SUCCESS"
	thirdPayQueryPaidStatus   = "3"
	thirdPayQueryFailedStatus = "-2"
	thirdPayHTTPTimeout       = 15 * time.Second
	maxThirdPayResponseSize   = 2 << 20 // 2MB
)

// ThirdPay implements the Dujiao-Next third-party payment protocol.
type ThirdPay struct {
	instanceID string
	config     map[string]string
	httpClient *http.Client
}

// NewThirdPay creates a ThirdPay provider.
// Required config keys: createOrderUrl, merchantId, merchantKey, reserve, notifyUrl.
// Optional keys: selectOrderUrl, refundOrderUrl, alipayType, wechatType, body.
func NewThirdPay(instanceID string, config map[string]string) (*ThirdPay, error) {
	cfg := normalizeThirdPayConfig(config)
	for _, k := range []string{"createOrderUrl", "merchantId", "merchantKey", "reserve", "notifyUrl"} {
		if strings.TrimSpace(cfg[k]) == "" {
			return nil, fmt.Errorf("thirdpay config missing required key: %s", k)
		}
	}
	if strings.TrimSpace(cfg["signType"]) == "" {
		cfg["signType"] = thirdPayDefaultSignType
	}
	if !strings.EqualFold(strings.TrimSpace(cfg["signType"]), thirdPayDefaultSignType) {
		return nil, fmt.Errorf("thirdpay only supports MD5 signType")
	}
	if strings.TrimSpace(cfg["wechatType"]) == "" {
		cfg["wechatType"] = thirdPayDefaultWechatType
	}
	if strings.TrimSpace(cfg["alipayType"]) == "" {
		cfg["alipayType"] = thirdPayDefaultAlipayType
	}
	return &ThirdPay{
		instanceID: instanceID,
		config:     cfg,
		httpClient: &http.Client{Timeout: thirdPayHTTPTimeout},
	}, nil
}

func normalizeThirdPayConfig(config map[string]string) map[string]string {
	cfg := make(map[string]string, len(config)+8)
	for k, v := range config {
		cfg[k] = strings.TrimSpace(v)
	}
	aliases := map[string]string{
		"create_order_url": "createOrderUrl",
		"select_order_url": "selectOrderUrl",
		"refund_order_url": "refundOrderUrl",
		"merchant_id":      "merchantId",
		"merchant_key":     "merchantKey",
		"notify_url":       "notifyUrl",
		"sign_type":        "signType",
		"wechat_type":      "wechatType",
		"alipay_type":      "alipayType",
	}
	for from, to := range aliases {
		if cfg[to] == "" && cfg[from] != "" {
			cfg[to] = cfg[from]
		}
	}
	return cfg
}

func (t *ThirdPay) Name() string        { return "ThirdPay" }
func (t *ThirdPay) ProviderKey() string { return payment.TypeThirdPay }
func (t *ThirdPay) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeAlipay, payment.TypeWxpay}
}

func (t *ThirdPay) MerchantIdentityMetadata() map[string]string {
	if t == nil {
		return nil
	}
	mchID := strings.TrimSpace(t.config["merchantId"])
	if mchID == "" {
		return nil
	}
	return map[string]string{"mch_id": mchID}
}

func (t *ThirdPay) CreatePayment(ctx context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	payType := t.resolvePayType(req.PaymentType)
	if payType == "" {
		return nil, fmt.Errorf("thirdpay unsupported payment type: %s", req.PaymentType)
	}
	notifyURL := strings.TrimSpace(req.NotifyURL)
	if notifyURL == "" {
		notifyURL = strings.TrimSpace(t.config["notifyUrl"])
	}
	if notifyURL == "" {
		return nil, fmt.Errorf("thirdpay notifyUrl is required")
	}
	clientIP := strings.TrimSpace(req.ClientIP)
	if clientIP == "" {
		clientIP = "127.0.0.1"
	}
	subject := strings.TrimSpace(req.Subject)
	if subject == "" {
		subject = req.OrderID
	}
	body := strings.TrimSpace(t.config["body"])
	if len([]rune(body)) > 50 {
		body = string([]rune(body)[:50])
	}

	params := map[string]string{
		"mch_id":       t.config["merchantId"],
		"type":         payType,
		"notify_url":   notifyURL,
		"out_trade_no": req.OrderID,
		"subject":      subject,
		"body":         body,
		"extra":        "payment_id=" + req.OrderID,
		"amount":       req.Amount,
		"ip":           clientIP,
	}
	params["sign"] = thirdPaySign(params, t.config["merchantKey"])
	params["sign_type"] = thirdPayDefaultSignType

	respBody, err := t.postForm(ctx, t.config["createOrderUrl"], params)
	if err != nil {
		return nil, fmt.Errorf("thirdpay create: %w", err)
	}
	raw, err := decodeThirdPayResponse(respBody)
	if err != nil {
		return nil, err
	}
	code, ok := thirdPayNumberCode(raw["code"])
	if !ok || code != thirdPaySuccessCode {
		return nil, fmt.Errorf("thirdpay error: %s", strings.TrimSpace(thirdPayString(raw, "msg")))
	}
	result := thirdPayMap(raw, "result")
	payURL := strings.TrimSpace(thirdPayString(result, "pay_url"))
	qrCode := strings.TrimSpace(thirdPayPickString(result, "qr_code", "qr_code "))
	if payURL == "" {
		payURL = strings.TrimSpace(thirdPayString(result, "url_scheme"))
	}
	if payURL == "" {
		if submit := strings.TrimSpace(thirdPayString(result, "submit")); submit != "" {
			payURL = buildThirdPaySubmitDataURL(submit)
		}
	}
	return &payment.CreatePaymentResponse{
		TradeNo: strings.TrimSpace(thirdPayPickString(result, "platform_sn", "sys_sn")),
		PayURL:  payURL,
		QRCode:  qrCode,
	}, nil
}

func (t *ThirdPay) QueryOrder(ctx context.Context, tradeNo string) (*payment.QueryOrderResponse, error) {
	if strings.TrimSpace(t.config["selectOrderUrl"]) == "" {
		return nil, fmt.Errorf("thirdpay selectOrderUrl is required")
	}
	tradeNo = strings.TrimSpace(tradeNo)
	if tradeNo == "" {
		return nil, fmt.Errorf("thirdpay query missing out_trade_no")
	}
	params := map[string]string{
		"mch_id":       t.config["merchantId"],
		"out_trade_no": tradeNo,
	}
	params["sign"] = thirdPaySign(params, t.config["merchantKey"])
	params["sign_type"] = thirdPayDefaultSignType

	respBody, err := t.postForm(ctx, t.config["selectOrderUrl"], params)
	if err != nil {
		return nil, fmt.Errorf("thirdpay query: %w", err)
	}
	raw, err := decodeThirdPayResponse(respBody)
	if err != nil {
		return nil, err
	}
	code, ok := thirdPayNumberCode(raw["code"])
	if !ok || code != thirdPaySuccessCode {
		return nil, fmt.Errorf("thirdpay query error: %s", strings.TrimSpace(thirdPayString(raw, "msg")))
	}
	result := thirdPayMap(raw, "result")
	status := payment.ProviderStatusPending
	switch strings.TrimSpace(thirdPayString(result, "status")) {
	case thirdPayQueryPaidStatus:
		status = payment.ProviderStatusPaid
	case thirdPayQueryFailedStatus:
		status = payment.ProviderStatusFailed
	}
	amount, _ := strconv.ParseFloat(strings.TrimSpace(thirdPayString(result, "amount")), 64)
	responseTradeNo := strings.TrimSpace(thirdPayPickString(result, "platform_sn", "sys_sn"))
	if responseTradeNo == "" {
		responseTradeNo = tradeNo
	}
	return &payment.QueryOrderResponse{
		TradeNo:  responseTradeNo,
		Status:   status,
		Amount:   amount,
		Metadata: t.MerchantIdentityMetadata(),
	}, nil
}

func (t *ThirdPay) VerifyNotification(_ context.Context, rawBody string, _ map[string]string) (*payment.PaymentNotification, error) {
	values, err := url.ParseQuery(rawBody)
	if err != nil {
		return nil, fmt.Errorf("thirdpay parse notify: %w", err)
	}
	params := make(map[string]string, len(values))
	for k := range values {
		params[k] = values.Get(k)
	}
	sign := strings.TrimSpace(params["sign"])
	if sign == "" {
		return nil, fmt.Errorf("thirdpay missing sign")
	}
	if !thirdPayVerifySign(params, t.config["merchantKey"], sign) {
		return nil, fmt.Errorf("thirdpay invalid signature")
	}
	status := thirdPayNotificationStatus(params)
	amount, _ := strconv.ParseFloat(strings.TrimSpace(thirdPayFirst(params, "amount", "money", "total_amount", "total_fee")), 64)
	metadata := t.MerchantIdentityMetadata()
	return &payment.PaymentNotification{
		TradeNo:  strings.TrimSpace(thirdPayFirst(params, "sys_sn", "platform_sn", "trade_no", "transaction_id")),
		OrderID:  strings.TrimSpace(thirdPayFirst(params, "out_trade_no", "outTradeNo", "order_no")),
		Amount:   amount,
		Status:   status,
		RawData:  rawBody,
		Metadata: metadata,
	}, nil
}

func thirdPayNotificationStatus(params map[string]string) string {
	status := strings.TrimSpace(thirdPayFirst(params, "status", "trade_status", "pay_status"))
	switch strings.ToUpper(status) {
	case thirdPayTradeSuccess, "TRADE_SUCCESS", "PAY_SUCCESS", "PAID":
		return payment.ProviderStatusSuccess
	}
	switch status {
	case thirdPayQueryPaidStatus:
		return payment.ProviderStatusSuccess
	default:
		return payment.ProviderStatusFailed
	}
}

func (t *ThirdPay) Refund(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
	return nil, fmt.Errorf("thirdpay refund is not implemented")
}

func (t *ThirdPay) resolvePayType(paymentType string) string {
	switch payment.GetBasePaymentType(strings.TrimSpace(paymentType)) {
	case payment.TypeAlipay:
		return strings.TrimSpace(t.config["alipayType"])
	case payment.TypeWxpay:
		return strings.TrimSpace(t.config["wechatType"])
	default:
		return ""
	}
}

func (t *ThirdPay) postForm(ctx context.Context, endpoint string, params map[string]string) ([]byte, error) {
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(endpoint), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if reserve := strings.TrimSpace(t.config["reserve"]); reserve != "" {
		req.Header.Set("reserve", reserve)
	}
	client := t.httpClient
	if client == nil {
		client = &http.Client{Timeout: thirdPayHTTPTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxThirdPayResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func decodeThirdPayResponse(body []byte) (map[string]any, error) {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	var raw map[string]any
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("thirdpay parse: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("thirdpay empty response")
	}
	return raw, nil
}

func thirdPaySign(params map[string]string, merchantKey string) string {
	return thirdPayMD5Hex(thirdPaySignContent(params) + strings.TrimSpace(merchantKey))
}

func thirdPayVerifySign(params map[string]string, merchantKey string, sign string) bool {
	got := strings.ToLower(strings.TrimSpace(thirdPaySign(params, merchantKey)))
	want := strings.ToLower(strings.TrimSpace(sign))
	return hmac.Equal([]byte(got), []byte(want))
}

func thirdPaySignContent(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		k = strings.TrimSpace(k)
		if k == "" || k == "sign" || k == "sign_type" {
			continue
		}
		if strings.TrimSpace(v) == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+strings.TrimSpace(params[k]))
	}
	return strings.Join(parts, "&")
}

func thirdPayMD5Hex(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

func thirdPayMap(raw map[string]any, key string) map[string]any {
	if raw == nil {
		return nil
	}
	if v, ok := raw[key].(map[string]any); ok {
		return v
	}
	return nil
}

func thirdPayString(raw map[string]any, key string) string {
	if raw == nil {
		return ""
	}
	switch v := raw[key].(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case float64:
		return fmt.Sprintf("%v", v)
	case int:
		return strconv.Itoa(v)
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprintf("%v", v)
	}
}

func thirdPayPickString(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(thirdPayString(raw, key)); v != "" {
			return v
		}
	}
	return ""
}

func thirdPayFirst(params map[string]string, keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(params[key]); v != "" {
			return v
		}
	}
	return ""
}

func thirdPayNumberCode(value any) (int, bool) {
	switch v := value.(type) {
	case json.Number:
		n, err := v.Int64()
		return int(n), err == nil
	case float64:
		return int(v), true
	case int:
		return v, true
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		return n, err == nil
	default:
		return 0, false
	}
}

func buildThirdPaySubmitDataURL(submit string) string {
	html := "<!doctype html><html><head><meta charset=\"utf-8\"></head><body>" + submit + "<script>if(document.forms.length){document.forms[0].submit();}</script></body></html>"
	return "data:text/html;charset=utf-8," + url.QueryEscape(html)
}
