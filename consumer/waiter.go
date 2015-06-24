package consumer

import (
	"time"
	"fmt"
)

type Waiter struct {
	*time.Ticker
}

func newWaiter() *Waiter {
	waiter := &Waiter{time.NewTicker(time.Millisecond * 250)}
	go waiter.run()
	return waiter
}

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
