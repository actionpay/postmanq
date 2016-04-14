package recipient

import (
	"fmt"
	"github.com/actionpay/postmanq/common"
	"github.com/actionpay/postmanq/logger"
	yaml "gopkg.in/yaml.v2"
	"net"
	"strings"
)

var (
	service *Service
)

func Inst() common.SendingService {
	if service == nil {
		service = new(Service)
	}
	return service
}

type Config struct {
	ListenerCount int    `yaml:"listenerCount"`
	Inbox         string `yaml:"inbox"`
	MxHostnames   []string
}

type Event struct {
	serverHostname   string
	serverMxHostname string
	clientHostname   []byte
	clientAddr       net.Addr
	conn             *net.TCPConn
	message          *common.MailMessage
}

type Service struct {
	Configs map[string]*Config `yaml:"postmans"`
}

func (s *Service) OnInit(event *common.ApplicationEvent) {
	err := yaml.Unmarshal(event.Data, s)
	if err == nil {
		for name, config := range s.Configs {
			s.init(config, name)
		}
	} else {
		logger.All().FailExitWithErr(err)
	}
}

func (s *Service) init(conf *Config, hostname string) {
	mxes, err := net.LookupMX(hostname)
	if err == nil {
		conf.MxHostnames = make([]string, len(mxes))
		for i, mx := range mxes {
			conf.MxHostnames[i] = strings.TrimRight(mx.Host, ".")
		}
		if conf.ListenerCount == 0 {
			conf.ListenerCount = common.DefaultWorkersCount
		}
	} else {
		logger.By(hostname).FailExit("recipient service - can't lookup mx for %s", hostname)
	}
}

func (s *Service) OnRun() {
	for hostname, conf := range s.Configs {
		for _, mxHostname := range conf.MxHostnames {
			tcpAddr := fmt.Sprintf("%s:2225", mxHostname)
			addr, err := net.ResolveTCPAddr("tcp", tcpAddr)
			if err == nil {
				logger.By(hostname).Info("recipient service - resolve %s success", tcpAddr)
				listener, err := net.ListenTCP("tcp", addr)
				if err == nil {
					logger.By(hostname).Info("recipient service - listen %s success", tcpAddr)
					go s.run(hostname, mxHostname, conf, listener)
				} else {
					logger.By(hostname).Warn("recipient service - can't listen %s, error - %v", tcpAddr, err)
				}
			} else {
				logger.By(hostname).Warn("recipient service - can't resolve %s, error - %v", tcpAddr, err)
			}
		}
	}
}

func (s *Service) run(hostname, mxHostname string, conf *Config, listener *net.TCPListener) {
	events := make(chan *Event)
	for i := 0; i < conf.ListenerCount; i++ {
		go newRecipient(i, events)
	}
	for {
		conn, err := listener.AcceptTCP()
		if err == nil {
			events <- &Event{
				serverHostname:   hostname,
				serverMxHostname: mxHostname,
				clientAddr:       conn.RemoteAddr(),
				conn:             conn,
			}
		} else {
			logger.By(hostname).Warn("recipient service - can't accept %s, error - %v", hostname, err)
		}
	}
}

func (s *Service) Events() chan *common.SendEvent {
	return nil
}

func (s *Service) OnFinish() {}
