package postmanq

import (
	"time"
	yaml "gopkg.in/yaml.v2"
	"sync"
)

var (
	limiter *Limiter
)

type Limiter struct {
	Limits map[string]*Limit `json:"limits"`
	mutex  *sync.Mutex
	ticker *time.Ticker
}

func NewLimiter() *Limiter {
	if (limiter == nil) {
		limiter = new(Limiter)
		limiter.Limits = make(map[string]*Limit)
		limiter.mutex = new(sync.Mutex)
		limiter.ticker = time.NewTicker(time.Second)
	}
	return limiter
}

func (this *Limiter) OnRegister() {}

func (this *Limiter) OnInit(event *InitEvent) {
	Debug("init limits...")
	err := yaml.Unmarshal(event.Data, this)
	if err == nil {
		for host, limit := range this.Limits {
			if duration, ok := limitDurations[limit.Type]; ok {
				limit.duration = duration
			}
			if bindingType, ok := limitBindingTypes[limit.Type]; ok {
				limit.bindingType = bindingType
			}
			Debug("create limit for %s with type %v and duration %v", host, limit.bindingType, limit.duration)
		}
		go this.checkLimits()
	} else {
		FailExitWithErr(err)
	}
}

func (this *Limiter) OnRun() {}

func (this *Limiter) OnFinish() {}

func (this *Limiter) OnSend(event *SendEvent) {
	this.mutex.Lock()
	Debug("check limit for mail#%d", event.Message.Id)
	if limit, ok := this.Limits[event.Message.HostnameTo]; ok {
		Debug("limit found for %s", event.Message.HostnameTo)
		if limit.isValidDuration(event.Message.CreatedDate) {
			limit.currentValue++
			Debug("limit current value %d, const value %d", limit.currentValue, limit.Value)
			if limit.currentValue > limit.Value {
				Debug("limit is exceeded for %s", event.Message.HostnameTo)
				event.DefaultPrevented = true
				event.Message.BindingType = limit.bindingType
				event.Message.Overlimit = true
				event.Message.Done <- false
			}
		} else {
			Debug("limit duration great then %v", limit.duration)
		}
	} else {
		Debug("limit not found for %s", event.Message.HostnameTo)
	}
	this.mutex.Unlock()
}

func (this *Limiter) checkLimits() {
	for now := range this.ticker.C {
		for host, limit := range this.Limits {
			if !limit.isValidDuration(now) {
				Debug("current limit value %d for %s, const value %d, reset", limit.currentValue, host, limit.Value)
				limit.currentValue = 0
				limit.modifyDate = time.Now()
			}
		}
	}
}

type LimitType string

const (
	LIMIT_SECOND_TYPE LimitType = "second"
	LIMIT_MINUTE_TYPE           = "minute"
	LIMIT_HOUR_TYPE             = "hour"
	LIMIT_DAY_TYPE              = "day"
)

var (
	limitDurations = map[LimitType]time.Duration {
		LIMIT_SECOND_TYPE: time.Second,
		LIMIT_MINUTE_TYPE: time.Minute,
		LIMIT_HOUR_TYPE  : time.Hour,
		LIMIT_DAY_TYPE   : time.Hour * 24,
	}

	limitBindingTypes = map[LimitType]DelayedBindingType {
		LIMIT_SECOND_TYPE: DELAYED_BINDING_SECOND,
		LIMIT_MINUTE_TYPE: DELAYED_BINDING_MINUTE,
		LIMIT_HOUR_TYPE  : DELAYED_BINDING_HOUR,
		LIMIT_DAY_TYPE   : DELAYED_BINDING_DAY,
	}
)

type Limit struct {
	Value        int                `json:"value"`
	Type         LimitType          `json:"type"`
	currentValue int
	duration     time.Duration
	modifyDate   time.Time
	bindingType  DelayedBindingType
}

func (this *Limit) isValidDuration(now time.Time) bool {
	return now.Sub(this.modifyDate) <= this.duration
}
