package internal

import (
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/zly-app/service/cron/types"
)

// CronTrigger Cron触发器
type CronTrigger struct {
	expression      string
	schedule        cron.Schedule
	nextExecuteTime time.Time
	mx              sync.Mutex
}

func NewCronTrigger(expression string) types.ITrigger {
	schedule, err := cron.ParseStandard(expression)
	if err != nil {
		panic(err)
	}

	return &CronTrigger{
		expression:      expression,
		schedule:        schedule,
		nextExecuteTime: schedule.Next(time.Now()),
	}
}

func (c *CronTrigger) TriggerType() types.TriggerType {
	return types.CronTriggerType
}

func (c *CronTrigger) Expression() string {
	return c.expression
}

func (c *CronTrigger) ResetClock() {
	c.mx.Lock()
	c.nextExecuteTime = c.schedule.Next(time.Now())
	c.mx.Unlock()
}

func (c *CronTrigger) MakeNextTriggerTime(t time.Time) (time.Time, bool) {
	c.mx.Lock()
	for t.Unix() >= c.nextExecuteTime.Unix() {
		c.nextExecuteTime = c.schedule.Next(c.nextExecuteTime)
	}
	t = c.nextExecuteTime
	c.mx.Unlock()
	return t, true
}

// OnceTrigger 一次性触发器
type OnceTrigger struct {
	expression  string
	executeTime time.Time
}

func NewOnceTrigger(t time.Time) types.ITrigger {
	return &OnceTrigger{
		expression:  t.Format(types.OnceTriggerTimeLayout),
		executeTime: t,
	}
}

func (o *OnceTrigger) TriggerType() types.TriggerType {
	return types.OnceTriggerType
}

func (o *OnceTrigger) Expression() string {
	return o.expression
}

func (o *OnceTrigger) ResetClock() {}

func (o *OnceTrigger) MakeNextTriggerTime(t time.Time) (time.Time, bool) {
	if t.Unix() < o.executeTime.Unix() {
		return o.executeTime, true
	}
	return t, false
}
