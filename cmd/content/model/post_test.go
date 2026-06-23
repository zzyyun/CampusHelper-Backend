package model

import "testing"

// TestPostStatus_Transition 测试帖子状态机的合法性。
// 这是内容服务的核心安全不变量：任何状态转移都必须走 allowedTransitions 图。
func TestPostStatus_Transition(t *testing.T) {
	cases := []struct {
		name    string
		from    PostStatus
		to      PostStatus
		wantErr bool
	}{
		// 合法转移：pending → published / rejected
		{"pending → published 合法", PostStatusPending, PostStatusPublished, false},
		{"pending → rejected  合法", PostStatusPending, PostStatusRejected, false},

		// 合法转移：published → closed / expired / retrieved / sold
		{"published → closed    合法", PostStatusPublished, PostStatusClosed, false},
		{"published → expired   合法", PostStatusPublished, PostStatusExpired, false},
		{"published → retrieved 合法", PostStatusPublished, PostStatusRetrieved, false},
		{"published → sold      合法", PostStatusPublished, PostStatusSold, false},

		// 幂等：相同状态视为合法
		{"published → published 幂等", PostStatusPublished, PostStatusPublished, false},
		{"pending → pending     幂等", PostStatusPending, PostStatusPending, false},

		// 非法转移
		{"pending → sold      非法", PostStatusPending, PostStatusSold, true},
		{"pending → retrieved 非法", PostStatusPending, PostStatusRetrieved, true},
		{"pending → closed    非法", PostStatusPending, PostStatusClosed, true},
		{"rejected → published 非法", PostStatusRejected, PostStatusPublished, true},
		{"rejected → closed    非法", PostStatusRejected, PostStatusClosed, true},
		{"closed → published   非法", PostStatusClosed, PostStatusPublished, true},
		{"expired → published  非法", PostStatusExpired, PostStatusPublished, true},
		{"retrieved → sold     非法", PostStatusRetrieved, PostStatusSold, true},
		{"sold → retrieved     非法", PostStatusSold, PostStatusRetrieved, true},

		// 未定义的源状态
		{"unspecified → published 非法", PostStatusUnspecified, PostStatusPublished, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.from.CanTransitionTo(tc.to)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("期望错误，实际为 nil")
				}
				if err != ErrInvalidTransition {
					t.Fatalf("期望 ErrInvalidTransition，实际为 %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("期望 nil，实际为 %v", err)
				}
			}
		})
	}
}

func TestErrInvalidTransition_Message(t *testing.T) {
	if ErrInvalidTransition == nil {
		t.Fatal("ErrInvalidTransition 不应为 nil")
	}
	if ErrInvalidTransition.Error() == "" {
		t.Fatal("ErrInvalidTransition.Error() 不应为空")
	}
}