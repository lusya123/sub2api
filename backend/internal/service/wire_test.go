package service

import (
	"errors"
	"testing"
	"time"

	"github.com/zeromicro/go-zero/core/collection"
)

func TestProvideTimingWheelService_ReturnsError(t *testing.T) {
	original := newTimingWheel
	t.Cleanup(func() { newTimingWheel = original })

	newTimingWheel = func(_ time.Duration, _ int, _ collection.Execute) (*collection.TimingWheel, error) {
		return nil, errors.New("boom")
	}

	svc, err := ProvideTimingWheelService()
	if err == nil {
		t.Fatalf("期望返回 error，但得到 nil")
	}
	if svc != nil {
		t.Fatalf("期望返回 nil svc，但得到非空")
	}
}

func TestProvideTimingWheelService_Success(t *testing.T) {
	svc, err := ProvideTimingWheelService()
	if err != nil {
		t.Fatalf("期望 err 为 nil，但得到: %v", err)
	}
	if svc == nil {
		t.Fatalf("期望 svc 非空，但得到 nil")
	}
	svc.Stop()
}

func TestProvideChannelHealthWiring_InjectsAsyncRecorder(t *testing.T) {
	recorder := ProvideAsyncChannelHealthRecorder(nil)
	t.Cleanup(func() { _ = recorder.Shutdown(time.Second) })

	gateway := &GatewayService{}
	openAIGateway := &OpenAIGatewayService{}
	antigravityGateway := &AntigravityGatewayService{}
	geminiMessagesCompat := &GeminiMessagesCompatService{}

	wiring := ProvideChannelHealthWiring(
		recorder,
		gateway,
		openAIGateway,
		antigravityGateway,
		geminiMessagesCompat,
	)
	if wiring == nil {
		t.Fatalf("期望 wiring marker 非空")
	}

	if gateway.channelHealthEnqueuer != recorder {
		t.Fatalf("GatewayService health recorder 未注入")
	}
	if openAIGateway.channelHealthEnqueuer != recorder {
		t.Fatalf("OpenAIGatewayService health recorder 未注入")
	}
	if antigravityGateway.channelHealthEnqueuer != recorder {
		t.Fatalf("AntigravityGatewayService health recorder 未注入")
	}
	if geminiMessagesCompat.channelHealthEnqueuer != recorder {
		t.Fatalf("GeminiMessagesCompatService health recorder 未注入")
	}
}
