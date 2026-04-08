package cron

import (
	"time"

	"github.com/zly-app/service/cron/internal"
	"github.com/zly-app/service/cron/types"
)

// ITrigger 触发器接口
type ITrigger = types.ITrigger

// TriggerType 触发器类型
type TriggerType = types.TriggerType

// CronTriggerType Cron触发器
const CronTriggerType = types.CronTriggerType

// OnceTriggerType 一次性触发器
const OnceTriggerType = types.OnceTriggerType

// OnceTriggerTimeLayout 一次性触发器时间样式
const OnceTriggerTimeLayout = types.OnceTriggerTimeLayout

// NewCronTrigger 创建 Cron 触发器
func NewCronTrigger(expression string) ITrigger {
	return internal.NewCronTrigger(expression)
}

// NewOnceTrigger 创建一次性触发器
func NewOnceTrigger(t time.Time) ITrigger {
	return internal.NewOnceTrigger(t)
}
