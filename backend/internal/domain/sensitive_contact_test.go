package domain

import "testing"

func TestLooksLikeContactContent(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		value string
		want  bool
	}{
		{name: "email", value: "请联系 review.user@example.com", want: true},
		{name: "mainland mobile", value: "手机号：13800138000", want: true},
		{name: "wechat assignment", value: "微信号: review_user_2026", want: true},
		{name: "telegram labeled", value: "Telegram ID @review_user", want: true},
		{name: "url encoded email", value: "review.user%40example.com", want: true},
		{name: "policy wording", value: "请不要提交手机号、邮箱或微信号。", want: false},
		{name: "order reference", value: "订单号 2026080313800138000 需要复核。", want: false},
		{name: "ordinary statement", value: "请复核账号限制所依据的事实。", want: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := LooksLikeContactContent(testCase.value); got != testCase.want {
				t.Fatalf("LooksLikeContactContent(%q) = %t, want %t", testCase.value, got, testCase.want)
			}
		})
	}
}
