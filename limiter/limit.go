package limiter

import (
	"github.com/Halfi/postmanq/common"
	"time"
)

// тип ограничения
type Kind string

const (
	SecondKind Kind = "second"
	MinuteKind      = "minute"
	HourKind        = "hour"
	DayKind         = "day"
)

var (
	// возможные промежутки времени для каждого ограничения
	limitDurations = map[Kind]time.Duration{
		SecondKind: time.Second,
		MinuteKind: time.Minute,
		HourKind:   time.Hour,
		DayKind:    time.Hour * 24,
	}
	// очереди для каждого ограничения
	limitBindingTypes = map[Kind]common.DelayedBindingType{
		SecondKind: common.SecondDelayedBinding,
		MinuteKind: common.MinuteDelayedBinding,
		HourKind:   common.HourDelayedBinding,
		DayKind:    common.DayDelayedBinding,
	}
)

// ограничение
type Limit struct {
	// максимально допустимое количество писем
	Value int32 `json:"value"`

	// тип ограничения
	Kind Kind `json:"type"`

	// текущее количество писем
	currentValue int32

	// промежуток времени, за который проверяется количество отправленных писем
	duration time.Duration

	// дата последнего обнуления количества отправленных писем
	modifyDate time.Time

	// тип очереди, в которую необходимо положить письмо, если превышено количество отправленных писем
	bindingType common.DelayedBindingType
}

// инициализирует значения по умолчанию
func (l *Limit) init() {
	if duration, ok := limitDurations[l.Kind]; ok {
		l.duration = duration
	}
	if bindingType, ok := limitBindingTypes[l.Kind]; ok {
		l.bindingType = bindingType
	}
}

// сигнализирует о том, что надо ли обнулять текущее количество отправленных писем
// если вернулось true, текущее количество отправленных писем не обнуляется
// если вернулось false, текущее количество отправленных писем обнуляется
func (l *Limit) isValidDuration(now time.Time) bool {
	return now.Sub(l.modifyDate) <= l.duration
}
