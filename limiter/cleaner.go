package limiter

import (
	"sync/atomic"
	"time"
)

// проверяет значения ограничений и обнуляет значения ограничений
type Cleaner struct{}

func newCleaner() {
	new(Cleaner).clean()
}

func (c *Cleaner) clean() {
	for now := range ticker.C {
		// смотрим все ограничения
		for _, limit := range service.Limits {
			// проверяем дату последнего изменения ограничения
			if !limit.isValidDuration(now) {
				// если дата последнего изменения выходит за промежуток времени для проверки
				// обнулям текущее количество отправленных писем
				atomic.StoreInt32(&limit.currentValue, 0)
				limit.modifyDate = time.Now()
			}
		}
	}
}
