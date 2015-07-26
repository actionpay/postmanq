package limiter

import (
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/logger"
	"sync/atomic"
)

type Limiter struct {
	id int
}

func newLimiter(id int) {
	limiter := &Limiter{id}
	limiter.run()
}

func (l *Limiter) run() {
	for event := range events {
		l.check(event)
	}
}

func (l *Limiter) check(event *common.SendEvent) {
	logger.Info("limiter#%d-%d check limit for mail", l.id, event.Message.Id)
	// пытаемся найти ограничения для почтового сервиса
	if limit, ok := service.Limits[event.Message.HostnameTo]; ok {
		logger.Debug("limiter#%d-%d found limit for %s", l.id, event.Message.Id, event.Message.HostnameTo)
		// если оно нашлось, проверяем, что отправка нового письма происходит в тот промежуток времени,
		// в который нам необходимо следить за ограничениями
		if limit.isValidDuration(event.Message.CreatedDate) {
			atomic.AddInt32(&limit.currentValue, 1)
			currentValue := atomic.LoadInt32(&limit.currentValue)
			logger.Debug("limiter#%d-%d detect current value %d, const value %d", l.id, event.Message.Id, currentValue, limit.Value)
			// если ограничение превышено
			if currentValue > limit.Value {
				logger.Debug("limiter#%d-%d current value is exceeded for %s", l.id, event.Message.Id, event.Message.HostnameTo)
				// определяем очередь, в которое переложем письмо
				event.Message.BindingType = limit.bindingType
				// говорим получателю, что у нас превышение ограничения,
				// разблокируем поток получателя
				event.Result <- common.OverlimitSendEventResult
				return
			}
		} else {
			logger.Debug("limiter#%d-%d duration great then %v", l.id, event.Message.Id, limit.duration)
		}
	} else {
		logger.Debug("limiter#%d-%d not found limit for %s", l.id, event.Message.Id, event.Message.HostnameTo)
	}
	event.Iterator.Next().(common.SendingService).Events() <- event
}
