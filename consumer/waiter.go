package consumer

import (
	"fmt"
	"time"
)

// ожидающий
type Waiter struct {
	*time.Ticker
}

// создает нового ожидающего
func newWaiter() *Waiter {
	waiter := &Waiter{time.NewTicker(time.Millisecond * 250)}
	go waiter.run()
	return waiter
}

// запускает нового ожидающего
func (w *Waiter) run() {
	commas := []string{
		".  ",
		" . ",
		"  .",
	}
	i := 0
	for {
		<-w.C
		fmt.Printf("\rgetting failure messages, please wait%s", commas[i])
		if i == 2 {
			i = 0
		} else {
			i++
		}
	}
}
