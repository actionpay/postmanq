package postmanq

import (
	"time"
	yaml "gopkg.in/yaml.v2"
	"sync"
)

var (
	limiter *Limiter
)

// сервис ограничений, следит за тем, чтобы почтовым сервисам не отправилось больше писем, чем нужно
type Limiter struct {
	Limits map[string]*Limit `json:"limits"` // ограничения для почтовых сервисов, в качестве ключа используется домен
	mutex  *sync.Mutex                       // семафор
	ticker *time.Ticker                      // таймер, работает каждую секунду
}

// создает сервис ограничений
func NewLimiter() *Limiter {
	if (limiter == nil) {
		limiter = new(Limiter)
		limiter.Limits = make(map[string]*Limit)
		limiter.mutex = new(sync.Mutex)
		limiter.ticker = time.NewTicker(time.Second)
	}
	return limiter
}

// инициализирует сервис
func (this *Limiter) OnInit(event *InitEvent) {
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
		// и сразу запускаем проверку значений ограничений
		go this.checkLimits()
	} else {
		FailExitWithErr(err)
	}
}

func (this *Limiter) OnRun() {}

func (this *Limiter) OnFinish() {}

// при отправке письма следит за тем,
// чтобы за указанный промежуток времени не было превышено количество отправленных писем
func (this *Limiter) OnSend(event *SendEvent) {
	this.mutex.Lock()
	Debug("check limit for mail#%d", event.Message.Id)
	// пытаемся найти ограничения для почтового сервиса
	if limit, ok := this.Limits[event.Message.HostnameTo]; ok {
		Debug("limit found for %s", event.Message.HostnameTo)
		// если оно нашлось, проверяем, что отправка нового письма происходит в тот промежуток времени,
		// в который нам необходимо следить за ограничениями
		if limit.isValidDuration(event.Message.CreatedDate) {
			limit.currentValue++
			Debug("limit current value %d, const value %d", limit.currentValue, limit.Value)
			// если ограничение превышено
			if limit.currentValue > limit.Value {
				Debug("limit is exceeded for %s", event.Message.HostnameTo)
				// отменяем последующую обработку письма
				event.DefaultPrevented = true
				// определяем очередь, в которое переложем письмо
				event.Message.BindingType = limit.bindingType
				// говорим получателю, что у нас превышение ограничения, для того,
				// чтобы он по другому обработал неудавшуюся отправку письма
				event.Message.Overlimit = true
				// разблокируем поток получателя
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

// проверяет значения ограничений и обнуляет значения ограничений
func (this *Limiter) checkLimits() {
	for now := range this.ticker.C {
		// смотрим все ограничения
		for host, limit := range this.Limits {
			// проверяем дату последнего изменения ограничения
			if !limit.isValidDuration(now) {
				// если дата последнего изменения выходит за промежуток времени для проверки
				// обнулям текущее количество отправленных писем
				Debug("current limit value %d for %s, const value %d, reset", limit.currentValue, host, limit.Value)
				limit.currentValue = 0
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
	Value        int                `json:"value"` // максимально допустимое количество писем
	Type         LimitType          `json:"type"`  // тип ограничения
	currentValue int                               // текущее количество писем
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
