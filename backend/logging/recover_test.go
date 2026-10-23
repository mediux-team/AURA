package logging

import (
	"testing"
	"time"
)

func TestGoRecoversAPanic(t *testing.T) {
	done := make(chan struct{})
	Go("test", func() {
		defer close(done)
		panic("test panic")
	})
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the goroutine didn't finish")
	}
}
