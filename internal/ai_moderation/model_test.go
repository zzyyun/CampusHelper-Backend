package ai_moderation

import (
	"context"
	"testing"
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
	if res.FallbackUsed != true {
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
	resp, err := svc.ModerateText(context.Background(), &ModerateTextRequest{Text: ""})
	if err != nil {
		t.Fatalf("empty text failed: %v", err)
	}
	if resp.Result != int32(ResultPass) {
		t.Errorf("empty text should return PASS, got %d", resp.Result)
	}
}

func TestService_ModerateText_Normal(t *testing.T) {
	loader := NewMockLoader("v1.0-test")
	svc := NewService(loader)
	resp, err := svc.ModerateText(context.Background(), &ModerateTextRequest{
		Text:    "今天天气真好",
		TraceID: "abc123",
		PostID:  42,
	})
	if err != nil {
		t.Fatalf("ModerateText failed: %v", err)
	}
	if resp.Result != int32(ResultPass) {
		t.Errorf("expected PASS, got %d", resp.Result)
	}
	if resp.ModelVersion != "v1.0-test" {
		t.Errorf("version mismatch: %s", resp.ModelVersion)
	}
	if resp.FallbackUsed != true {
		t.Error("mock mode should set FallbackUsed=true")
	}
}
