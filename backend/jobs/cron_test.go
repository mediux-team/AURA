package jobs

import (
	"testing"

	"github.com/robfig/cron/v3"
)

func TestCronRecoversAPanic(t *testing.T) {
	// The same chain the scheduler uses, a panicking job is logged and doesn't stop aura
	job := cron.NewChain(cron.Recover(cronLogger{})).Then(cron.FuncJob(func() { panic("test panic") }))
	job.Run()
}
