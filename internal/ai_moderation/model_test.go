package ai_moderation

import (
	"context"
	"testing"

	ai_moderation_pb "go_projects/praProject1/PB/pb/ai_moderation_pb"
)

func TestMockLoader_Infer(t *testing.T) {
	loader := NewMockLoader("v1.0-test")
	res, err := loader.Infer(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("Infer failed: %v", err)
	}
	if res.Result != ResultPass {
		t.Errorf("expected ResultPass, got %v", res.Result)
	}
	if res.Confidence != 1.0 {
		t.Errorf("expected confidence=1.0, got %f", res.Confidence)
	}
	if !res.FallbackUsed {
		t.Error("mock loader should set FallbackUsed=true")
	}
	if res.ModelVersion != "v1.0-test" {
		t.Errorf("version mismatch: %s", res.ModelVersion)
	}
}

func TestModelConfig_Validate(t *testing.T) {
	c := ModelConfig{ModelVersion: ""}
	if err := c.Validate(); err == nil {
		t.Error("expected error for empty version")
	}

	c = ModelConfig{ModelVersion: "v1", Enabled: true, ModelPath: ""}
	if err := c.Validate(); err == nil {
		t.Error("expected error when enabled=true and ModelPath empty")
	}

	c = ModelConfig{ModelVersion: "v1", Enabled: false}
	if err := c.Validate(); err != nil {
		t.Errorf("mock mode should validate, got: %v", err)
	}
	if c.TimeoutMs != 800 {
		t.Errorf("default timeout should be 800, got %d", c.TimeoutMs)
	}
}

func TestNewModelLoader_MockMode(t *testing.T) {
	cfg := ModelConfig{
		ModelVersion: "v1.0-mock",
		Enabled:      false,
		TimeoutMs:    500,
	}
	loader, err := NewModelLoader(cfg)
	if err != nil {
		t.Fatalf("mock loader init failed: %v", err)
	}
	defer loader.Close()

	if loader.Version() != "v1.0-mock" {
		t.Errorf("version mismatch: %s", loader.Version())
	}
}

func TestService_ModerateText_Empty(t *testing.T) {
	loader := NewMockLoader("v1.0-test")
	svc := NewService(loader)
	resp, err := svc.ModerateText(context.Background(), &ai_moderation_pb.ModerateTextRequest{Text: ""})
	if err != nil {
		t.Fatalf("empty text failed: %v", err)
	}
	if resp.Result != ai_moderation_pb.ModerateTextResponse_PASS {
		t.Errorf("empty text should return PASS, got %v", resp.Result)
	}
}

func TestService_ModerateText_Normal(t *testing.T) {
	loader := NewMockLoader("v1.0-test")
	svc := NewService(loader)
	resp, err := svc.ModerateText(context.Background(), &ai_moderation_pb.ModerateTextRequest{
		Text:    "今天天气真好",
		TraceId: "abc123",
		PostId:  42,
	})
	if err != nil {
		t.Fatalf("ModerateText failed: %v", err)
	}
	if resp.Result != ai_moderation_pb.ModerateTextResponse_PASS {
		t.Errorf("expected PASS, got %v", resp.Result)
	}
	if resp.ModelVersion != "v1.0-test" {
		t.Errorf("version mismatch: %s", resp.ModelVersion)
	}
	if !resp.FallbackUsed {
		t.Error("mock mode should set FallbackUsed=true")
	}
}

func TestService_HealthCheck(t *testing.T) {
	loader := NewMockLoader("v1.0-health")
	svc := NewServiceWithMode(loader, "mock", 4)
	resp, err := svc.HealthCheck(context.Background(), nil)
	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}
	if resp.Status != ai_moderation_pb.HealthCheckResponse_SERVING {
		t.Errorf("expected SERVING, got %v", resp.Status)
	}
	if !resp.ModelLoaded {
		t.Error("model should be loaded")
	}
	if resp.Mode != "mock" {
		t.Errorf("expected mode=mock, got %s", resp.Mode)
	}
	if resp.IntraOpThreads != 4 {
		t.Errorf("expected threads=4, got %d", resp.IntraOpThreads)
	}
}

func TestModerateTextResultToPb(t *testing.T) {
	tests := []struct {
		input    Result
		expected ai_moderation_pb.ModerateTextResponse_Result
	}{
		{ResultPass, ai_moderation_pb.ModerateTextResponse_PASS},
		{ResultReview, ai_moderation_pb.ModerateTextResponse_REVIEW},
		{ResultBlock, ai_moderation_pb.ModerateTextResponse_BLOCK},
		{Result(99), ai_moderation_pb.ModerateTextResponse_REVIEW}, // 兜底
	}
	for _, tt := range tests {
		got := moderateTextResultToPb(tt.input)
		if got != tt.expected {
			t.Errorf("input=%d: expected %v, got %v", tt.input, tt.expected, got)
		}
	}
}

func TestService_DegradedResponse(t *testing.T) {
	loader := NewMockLoader("v1.0-degraded")
	svc := NewService(loader)
	resp := svc.degradedResponse("test reason")
	if resp.Result != ai_moderation_pb.ModerateTextResponse_PASS {
		t.Errorf("degraded should fallback to PASS, got %v", resp.Result)
	}
	if resp.Status != ai_moderation_pb.ModerateTextResponse_DEGRADED {
		t.Errorf("expected DEGRADED status, got %v", resp.Status)
	}
	if !resp.FallbackUsed {
		t.Error("FallbackUsed should be true")
	}
}