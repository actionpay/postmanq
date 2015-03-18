package postmanq

import (
	"time"
	yaml "gopkg.in/yaml.v2"
	"sync/atomic"
)

var (
	limiter *Limiter
)

// сервис ограничений, следит за тем, чтобы почтовым сервисам не отправилось больше писем, чем нужно
type Limiter struct {
	LimitersCount int               `yaml:"workers"`
	Limits        map[string]*Limit `json:"limits"` // ограничения для почтовых сервисов, в качестве ключа используется домен
	ticker        *time.Ticker                      // таймер, работает каждую секунду
	events        chan *SendEvent
}

// создает сервис ограничений
func LimiterOnce() *Limiter {
	if (limiter == nil) {
		limiter = new(Limiter)
		limiter.Limits = make(map[string]*Limit)
		limiter.ticker = time.NewTicker(time.Second)
		limiter.events = make(chan *SendEvent)
	}
	return limiter
}

// инициализирует сервис
func (this *Limiter) OnInit(event *ApplicationEvent) {
	Debug("init limits...")
	err := yaml.Unmarshal(event.Data, this)
	if err == nil {
		// инициализируем ограничения
		for host, limit := range this.Limits {
			if duration, ok := limitDurations[limit.Type]; ok {
				limit.duration = duration
			}
			if bindingType, ok := limitBindingTypes[limit.Type]; ok {
				limit.bindingType = bindingType
			}
			Debug("create limit for %s with type %v and duration %v", host, limit.bindingType, limit.duration)
		}
		if this.LimitersCount == 0 {
			this.LimitersCount = defaultWorkersCount
		}
	} else {
		FailExitWithErr(err)
	}
}

func (this *Limiter) OnRun() {
	// сразу запускаем проверку значений ограничений
	go this.checkLimitValues()
	for i := 0;i < this.LimitersCount; i++ {
		go this.checkLimits(i + 1)
	}
}

func (this *Limiter) checkLimits(id int) {
	for event := range this.events {
		this.doChecking(id, event)
	}
}

func (this *Limiter) doChecking(id int, event *SendEvent) {
	Info("limiter#%d check limit for mail#%d", id, event.Message.Id)
	// пытаемся найти ограничения для почтового сервиса
	if limit, ok := this.Limits[event.Message.HostnameTo]; ok {
		Debug("limiter#%d found config for %s", id, event.Message.HostnameTo)
		// если оно нашлось, проверяем, что отправка нового письма происходит в тот промежуток времени,
		// в который нам необходимо следить за ограничениями
		if limit.isValidDuration(event.Message.CreatedDate) {
			atomic.AddInt32(&limit.currentValue, 1)
			currentValue := atomic.LoadInt32(&limit.currentValue)
			Debug("limiter#%d get current value %d, const value %d", id, currentValue, limit.Value)
			// если ограничение превышено
			if currentValue > limit.Value {
				Debug("limiter#%d current value is exceeded for %s", id, event.Message.HostnameTo)
				// определяем очередь, в которое переложем письмо
				event.Message.BindingType = limit.bindingType
				// говорим получателю, что у нас превышение ограничения,
				// разблокируем поток получателя
				event.Result <- SEND_EVENT_RESULT_OVERLIMIT
				return
			}
		} else {
			Debug("limiter#%d duration great then %v", id, limit.duration)
		}
	} else {
		Debug("limiter#%d not found for %s", id, event.Message.HostnameTo)
	}
	connector.events <- event
}

func (this *Limiter) OnFinish() {
	close(this.events)
}

// при отправке письма следит за тем,
// чтобы за указанный промежуток времени не было превышено количество отправленных писем
func (this *Limiter) OnSend(event *SendEvent) {}

// проверяет значения ограничений и обнуляет значения ограничений
func (this *Limiter) checkLimitValues() {
	for now := range this.ticker.C {
		// смотрим все ограничения
		for host, limit := range this.Limits {
			// проверяем дату последнего изменения ограничения
			if !limit.isValidDuration(now) {
				// если дата последнего изменения выходит за промежуток времени для проверки
				// обнулям текущее количество отправленных писем
				Debug("current limit value %d for %s, const value %d, reset...", limit.currentValue, host, limit.Value)
				atomic.StoreInt32(&limit.currentValue, 0)
				limit.modifyDate = time.Now()
			}
		}
	}
}

// тип ограничения
type LimitType string

const (
	LIMIT_SECOND_TYPE LimitType = "second"
	LIMIT_MINUTE_TYPE           = "minute"
	LIMIT_HOUR_TYPE             = "hour"
	LIMIT_DAY_TYPE              = "day"
)

var (
	// возможные промежутки времени для каждого ограничения
	limitDurations = map[LimitType]time.Duration {
		LIMIT_SECOND_TYPE: time.Second,
		LIMIT_MINUTE_TYPE: time.Minute,
		LIMIT_HOUR_TYPE  : time.Hour,
		LIMIT_DAY_TYPE   : time.Hour * 24,
	}
	// очереди для каждого ограничения
	limitBindingTypes = map[LimitType]DelayedBindingType {
		LIMIT_SECOND_TYPE: DELAYED_BINDING_SECOND,
		LIMIT_MINUTE_TYPE: DELAYED_BINDING_MINUTE,
		LIMIT_HOUR_TYPE  : DELAYED_BINDING_HOUR,
		LIMIT_DAY_TYPE   : DELAYED_BINDING_DAY,
	}
)

// ограничение
type Limit struct {
	Value        int32              `json:"value"` // максимально допустимое количество писем
	Type         LimitType          `json:"type"`  // тип ограничения
	currentValue int32                             // текущее количество писем
	duration     time.Duration                     // промежуток времени, за который проверяется количество отправленных писем
	modifyDate   time.Time                         // дата последнего обнуления количества отправленных писем
	bindingType  DelayedBindingType                // тип очереди, в которую необходимо положить письмо, если превышено количество отправленных писем
}

// сигнализирует о том, что надо ли обнулять текущее количество отправленных писем
// если вернулось true, текущее количество отправленных писем не обнуляется
// если вернулось false, текущее количество отправленных писем обнуляется
func (this *Limit) isValidDuration(now time.Time) bool {
	return now.Sub(this.modifyDate) <= this.duration
}
