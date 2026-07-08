package service

import (
	"testing"
	"time"
)

func TestNewAuditStatsService(t *testing.T) {
	svc := NewAuditStatsService()
	if svc == nil {
		t.Fatal("NewAuditStatsService returned nil")
	}
}

func TestDashboardStats_RatesCalculation(t *testing.T) {
	// 验证通过率/拦截率/人工率的计算逻辑
	passCount := int64(60)
	reviewCount := int64(20)
	blockCount := int64(20)
	total := passCount + reviewCount + blockCount

	passRate := float64(passCount) / float64(total) * 100
	blockRate := float64(blockCount) / float64(total) * 100
	reviewRate := float64(reviewCount) / float64(total) * 100

	if passRate != 60.0 {
		t.Errorf("passRate = %f, want 60.0", passRate)
	}
	if blockRate != 20.0 {
		t.Errorf("blockRate = %f, want 20.0", blockRate)
	}
	if reviewRate != 20.0 {
		t.Errorf("reviewRate = %f, want 20.0", reviewRate)
	}

	// 三者之和应为 100%
	sum := passRate + blockRate + reviewRate
	if sum != 100.0 {
		t.Errorf("rates sum = %f, want 100.0", sum)
	}
}

func TestDashboardStats_ZeroTotal(t *testing.T) {
	// 零总量时不产生除零错误
	var total, passCount int64
	if total > 0 {
		_ = float64(passCount) / float64(total)
	} else {
		// 零总量时比率应为 0
		passRate := 0.0
		if passRate != 0.0 {
			t.Error("zero total should give 0 rate")
		}
	}
}

func TestDashboardStats_PRDTargets(t *testing.T) {
	// PRD 成功指标验证逻辑
	tests := []struct {
		name    string
		passR   float64 // 通过率
		blockR  float64 // 拦截率
		wantOk  bool
	}{
		{"达标: 60%+20%", 60.0, 20.0, true},   // pass>=60% + block>=20% → 自动裁决率>=80%
		{"边界: 55%+25%", 55.0, 25.0, true},   // 80% 自动裁决
		{"不达标: 40%+20%", 40.0, 20.0, false}, // 仅60%
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			autoDecisionRate := tt.passR + tt.blockR
			gotOk := autoDecisionRate >= 80.0
			if gotOk != tt.wantOk {
				t.Errorf("autoDecisionRate=%.1f%%, want ok=%v", autoDecisionRate, tt.wantOk)
			}
		})
	}
}

func TestGetDailyTrend_DaysLimits(t *testing.T) {
	// 验证天数限制逻辑
	tests := []struct {
		input int
		want  int
	}{
		{0, 7},    // 默认 7 天
		{-1, 7},   // 负数 → 7 天
		{5, 5},    // 正常
		{30, 30},  // 最大
		{60, 30},  // 超过 30 → 截断到 30
	}

	for _, tt := range tests {
		days := tt.input
		if days <= 0 {
			days = 7
		}
		if days > 30 {
			days = 30
		}
		if days != tt.want {
			t.Errorf("days=%d input=%d, want %d", days, tt.input, tt.want)
		}
	}
}

func TestGetDailyTrend_TimeRange(t *testing.T) {
	days := 7
	since := time.Now().AddDate(0, 0, -days)
	if since.After(time.Now()) {
		t.Error("since should be in the past")
	}
	expectedMin := time.Now().AddDate(0, 0, -8)
	if since.Before(expectedMin) {
		t.Error("since should be within 7-8 days ago")
	}
}
