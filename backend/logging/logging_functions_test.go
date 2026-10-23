package logging

import (
	"context"
	"testing"
)

func TestSubActionWithoutCurrentAction(t *testing.T) {
	// A context without an action still gives a sub-action that can be used
	ctx, logAction := AddSubActionToContext(context.Background(), "test", LevelInfo)
	if logAction == nil {
		t.Fatal("expected a sub-action")
	}
	logAction.SetError("failed", "", nil)
	if logAction.Error == nil || logAction.Error.Message != "failed" {
		t.Errorf("Error = %+v", logAction.Error)
	}
	if CurrentActionFromContext(ctx) != logAction {
		t.Error("the sub-action should be the current action")
	}
}
