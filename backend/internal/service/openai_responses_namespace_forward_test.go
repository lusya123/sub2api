//go:build unit

package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// Codex can carry the same native tool declaration at the top level or inside
// Responses Lite additional_tools. Both must match replayed call identities.
func TestOpenAIGatewayService_APIKeyNamespaceContinuation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const declarations = `[{"type":"namespace","name":"functions","tools":[
		{"type":"function","name":"read_file","parameters":{"type":"object"}},
		{"type":"custom","name":"exec","format":{"type":"text"}}
	]}]`
	const history = `
		{"type":"function_call","call_id":"call_read","name":"read_file","namespace":"functions","arguments":"{}"},
		{"type":"function_call_output","call_id":"call_read","output":"ok"},
		{"type":"custom_tool_call","call_id":"call_exec","name":"exec","namespace":"functions","input":"print(1)"},
		{"type":"custom_tool_call_output","call_id":"call_exec","output":"1"},
		{"type":"message","namespace":"residual","role":"user","content":"Continue with another command."}`
	for _, carrier := range []string{"top_level", "additional_tools"} {
		for _, passthrough := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/passthrough_%t", carrier, passthrough), func(t *testing.T) {
				var body string
				if carrier == "top_level" {
					body = fmt.Sprintf(`{"model":"gpt-5.6-sol","stream":true,"store":false,"tools":%s,"input":[%s]}`, declarations, history)
				} else {
					body = fmt.Sprintf(`{"model":"gpt-5.6-sol","stream":true,"store":false,"input":[{"type":"additional_tools","role":"developer","tools":%s},%s]}`, declarations, history)
				}
				const call = `{"type":"custom_tool_call","id":"ctc_next","call_id":"call_next","name":"exec","namespace":"functions","input":"print(2)","status":"completed"}`
				stream := "event: response.output_item.added\ndata: {\"type\":\"response.output_item.added\",\"output_index\":0,\"item\":" + call + "}\n\n" +
					"event: response.output_item.done\ndata: {\"type\":\"response.output_item.done\",\"output_index\":0,\"item\":" + call + "}\n\n" +
					"event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_next\",\"status\":\"completed\",\"output\":[" + call + "],\"usage\":{\"input_tokens\":20,\"output_tokens\":5}}}\n\n" +
					"data: [DONE]\n\n"
				upstream := &httpUpstreamRecorder{resp: &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
					Body:       io.NopCloser(strings.NewReader(stream)),
				}}
				svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream, cache: &stubGatewayCache{}, toolCorrector: NewCodexToolCorrector()}
				account := &Account{
					ID: 124, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
					Status: StatusActive, Schedulable: true, Concurrency: 1,
					Credentials: map[string]any{"api_key": "test-key", "base_url": "https://api.example.test"},
					Extra:       map[string]any{"openai_passthrough": passthrough, "openai_responses_supported": true},
				}
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
				c.Request.Header.Set("User-Agent", "codex_cli_rs/0.153.4")
				if carrier == "additional_tools" {
					c.Request.Header.Set(responsesLiteHeader, "true")
				}
				SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)

				_, err := svc.Forward(context.Background(), c, account, []byte(body))

				require.NoError(t, err)
				require.Equal(t, "functions", gjson.GetBytes(upstream.lastBody, `input.#(type=="function_call").namespace`).String())
				require.Equal(t, "functions", gjson.GetBytes(upstream.lastBody, `input.#(type=="custom_tool_call").namespace`).String())
				require.Equal(t, "exec", gjson.GetBytes(upstream.lastBody, `input.#(type=="custom_tool_call").name`).String())
				require.False(t, gjson.GetBytes(upstream.lastBody, `input.#(type=="message").namespace`).Exists())
				declarationPath := `tools.0.name`
				if carrier == "additional_tools" {
					declarationPath = `input.#(type=="additional_tools").tools.0.name`
				}
				require.Equal(t, "functions", gjson.GetBytes(upstream.lastBody, declarationPath).String())
				returnedCalls := 0
				for _, line := range strings.Split(recorder.Body.String(), "\n") {
					if !strings.HasPrefix(line, "data: ") {
						continue
					}
					payload := strings.TrimPrefix(line, "data: ")
					item := gjson.Get(payload, "item")
					if !item.Exists() {
						item = gjson.Get(payload, "response.output.0")
					}
					if item.Exists() {
						require.Equal(t, "custom_tool_call", item.Get("type").String())
						require.Equal(t, "functions", item.Get("namespace").String())
						require.Equal(t, "exec", item.Get("name").String())
						returnedCalls++
					}
				}
				require.Equal(t, 3, returnedCalls, "added, done, and completed events must keep custom tool identity")
			})
		}
	}
}

func TestOpenAIGatewayService_APIKeyCompactStillStripsCallNamespaces(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		t.Run(fmt.Sprintf("passthrough_%t", passthrough), func(t *testing.T) {
			body := []byte(`{"model":"gpt-5.6-sol","input":[
				{"type":"additional_tools","role":"developer","tools":[{"type":"namespace","name":"functions","tools":[{"type":"custom","name":"exec"}]}]},
				{"type":"custom_tool_call","call_id":"call_exec","name":"exec","namespace":"functions","input":"print(1)"},
				{"type":"custom_tool_call_output","call_id":"call_exec","output":"1"}
			]}`)
			// Stop at the external HTTP boundary: compact's response format is not
			// part of this test; the outgoing cleanup policy is.
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"recorded request","type":"invalid_request_error"}}`)),
			}}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream, cache: &stubGatewayCache{}, toolCorrector: NewCodexToolCorrector()}
			account := &Account{
				ID: 124, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
				Status: StatusActive, Schedulable: true, Concurrency: 1,
				Credentials: map[string]any{"api_key": "test-key", "base_url": "https://api.example.test"},
				Extra:       map[string]any{"openai_passthrough": passthrough, "openai_responses_supported": true},
			}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", strings.NewReader(string(body)))
			SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)

			_, err := svc.Forward(context.Background(), c, account, body)

			require.Error(t, err)
			require.NotNil(t, upstream.lastReq)
			require.Equal(t, "/v1/responses/compact", upstream.lastReq.URL.Path)
			require.Equal(t, "exec", gjson.GetBytes(upstream.lastBody, `input.#(type=="custom_tool_call").name`).String())
			require.False(t, gjson.GetBytes(upstream.lastBody, `input.#(type=="custom_tool_call").namespace`).Exists())
		})
	}
}
