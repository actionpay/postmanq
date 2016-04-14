package main

import (
	"github.com/actionpay/postmanq/common"
	"github.com/actionpay/postmanq/logger"
	"net"
	"net/smtp"
	"runtime"
	"time"
)

func main() {
	common.DefaultWorkersCount = runtime.NumCPU()
	common.DefaultWorkersCount = 1
	logger.Inst()

	logger.By("localhost").Info("start!")
	tcpAddr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort("localhost", "0"))
	if err == nil {
		logger.By("localhost").Info("resolve tcp addr localhost")
		dialer := &net.Dialer{
			LocalAddr: tcpAddr,
			Timeout:   time.Second,
		}
		hostname := net.JoinHostPort("localhost", "2225")
		connection, err := dialer.Dial("tcp", hostname)
		if err == nil {
			logger.By("localhost").Info("dial localhost:2225")
			client, err := smtp.NewClient(connection, "example.com")
			if err == nil {
				logger.By("localhost").Info("create client")
				err := client.Hello("example.com")
				logger.By("localhost").Info("%v", err)

				err = client.Mail("sender@example.com")
				logger.By("localhost").Info("%v", err)

				err = client.Rcpt("recipient@example.com")
				logger.By("localhost").Info("%v", err)
			} else {
				logger.By("localhost").Info("can't create client")
			}
		} else {
			logger.By("localhost").Info("can't dial localhost:2225")
		}
	} else {
		logger.By("localhost").Info("can't resolve tcp addr localhost")
	}
	time.Sleep(time.Second * 5)
}
