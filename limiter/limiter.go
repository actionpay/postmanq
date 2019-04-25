package limiter

import (
	"github.com/Halfi/postmanq/common"
	"github.com/Halfi/postmanq/logger"
	"sync/atomic"
)

// ограничитель, проверяет количество отправленных писем почтовому сервису
type Limiter struct {
	// идентификатор для логов
	id int
}

// создает нового ограничителя
func newLimiter(id int) {
	limiter := &Limiter{id}
	limiter.run()
}

// запускает ограничителя
func (l *Limiter) run() {
	for event := range events {
		l.check(event)
	}
}

// проверяет количество отправленных писем почтовому сервису
// если количество превышено, отправляет письмо в отложенную очередь
func (l *Limiter) check(event *common.SendEvent) {
	logger.By(event.Message.HostnameFrom).Info("limiter#%d-%d check limit for mail", l.id, event.Message.Id)
	limit := service.getLimit(event.Message.HostnameFrom, event.Message.HostnameTo)
	// пытаемся найти ограничения для почтового сервиса
	if limit == nil {
		logger.By(event.Message.HostnameFrom).Debug("limiter#%d-%d not found limit for %s", l.id, event.Message.Id, event.Message.HostnameTo)
	} else {
		logger.By(event.Message.HostnameFrom).Debug("limiter#%d-%d found limit for %s", l.id, event.Message.Id, event.Message.HostnameTo)
		// если оно нашлось, проверяем, что отправка нового письма происходит в тот промежуток времени,
		// в который нам необходимо следить за ограничениями
		if limit.isValidDuration(event.Message.CreatedDate) {
			atomic.AddInt32(&limit.currentValue, 1)
			currentValue := atomic.LoadInt32(&limit.currentValue)
			logger.By(event.Message.HostnameFrom).Debug("limiter#%d-%d detect current value %d, const value %d", l.id, event.Message.Id, currentValue, limit.Value)
			// если ограничение превышено
			if currentValue > limit.Value {
				logger.By(event.Message.HostnameFrom).Debug("limiter#%d-%d current value is exceeded for %s", l.id, event.Message.Id, event.Message.HostnameTo)
				// определяем очередь, в которое переложем письмо
				event.Message.BindingType = limit.bindingType
				// говорим получателю, что у нас превышение ограничения,
				// разблокируем поток получателя
				event.Result <- common.OverlimitSendEventResult
				return
			}
		} else {
			logger.By(event.Message.HostnameFrom).Debug("limiter#%d-%d duration great then %v", l.id, event.Message.Id, limit.duration)
		}
	}
	event.Iterator.Next().(common.SendingService).Events() <- event
}
