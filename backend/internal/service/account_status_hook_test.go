package service

import (
	"context"
	"testing"
)

// 状态变更钩子只在「状态真的变了」且不是反向同步触发时才会回调。
func TestAccountStatusChangeHookBypass(t *testing.T) {
	base := context.Background()

	if accountStatusSyncBypassed(base) {
		t.Fatal("普通 context 不应被标记为反向同步")
	}
	if !accountStatusSyncBypassed(WithAccountStatusSyncBypass(base)) {
		t.Fatal("WithAccountStatusSyncBypass 应标记 context")
	}

	// bypass context 不得污染父 context。
	if accountStatusSyncBypassed(base) {
		t.Fatal("bypass 标记泄漏到了父 context")
	}
}
