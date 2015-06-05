package limit

import (
	"github.com/AdOnWeb/postmanq/log"
	"sync/atomic"
	"time"
)

// проверяет значения ограничений и обнуляет значения ограничений
type Cleaner struct{}

func (c *Cleaner) clean() {
	for now := range ticker.C {
		// смотрим все ограничения
		for host, limit := range service.Limits {
			// проверяем дату последнего изменения ограничения
			if !limit.isValidDuration(now) {
				// если дата последнего изменения выходит за промежуток времени для проверки
				// обнулям текущее количество отправленных писем
				log.Debug("current limit value %d for %s, const value %d, reset...", limit.currentValue, host, limit.Value)
				atomic.StoreInt32(&limit.currentValue, 0)
				limit.modifyDate = time.Now()
			}
		}
	}
}
