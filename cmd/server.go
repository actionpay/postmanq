package main

import (
	"github.com/actionpay/postmanq/common"
	"github.com/actionpay/postmanq/logger"
	"github.com/actionpay/postmanq/recipient"
	"runtime"
)

func main() {
	common.DefaultWorkersCount = runtime.NumCPU()
	logger.Inst()

	conf := &recipient.Config{
		ListenerCount: 10,
		MxHostnames:   []string{"localhost"},
	}
	service := recipient.Inst()
	service.(*recipient.Service).Configs = map[string]*recipient.Config{
		"localhost": conf,
	}
	service.OnRun()

	ch := make(chan bool)
	<-ch
}
