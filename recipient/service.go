package recipient

import (
	"fmt"
	"net"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Halfi/postmanq/common"
	"github.com/Halfi/postmanq/logger"
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

	// hostname, на котором будет слушаться 25 порт
	MXHostname string `yaml:"mxHostname"`

	mxHostnames []string
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
			if config.MXHostname != "" {
				name = config.MXHostname
			}
			s.init(config, name)
		}
	} else {
		logger.All().FailExitErr(err)
	}
}

func (s *Service) init(conf *Config, hostname string) {
	mxes, err := net.LookupMX(hostname)
	if err == nil {
		conf.mxHostnames = make([]string, len(mxes))
		for i, mx := range mxes {
			conf.mxHostnames[i] = strings.TrimRight(mx.Host, ".")
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
		for _, mxHostname := range conf.mxHostnames {
			tcpAddr := fmt.Sprintf("%s:25", mxHostname)
			addr, err := net.ResolveTCPAddr("tcp", tcpAddr)
			if err == nil {
				logger.By(hostname).Info("recipient service - resolve %s success", tcpAddr)
				listener, err := net.ListenTCP("tcp", addr)
				if err == nil {
					go s.run(hostname, mxHostname, conf, listener)
				} else {
					logger.By(hostname).WarnWithErr(err, "recipient service - can't listen %s", tcpAddr)
				}
			} else {
				logger.By(hostname).WarnWithErr(err, "recipient service - can't resolve %s", tcpAddr)
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
			events <- &Event{serverHostname: hostname, serverMxHostname: mxHostname, clientAddr: conn.RemoteAddr(), conn: conn}
		} else {
			logger.By(hostname).WarnWithErr(err, "recipient service - can't accept %s", hostname)
		}
	}
}

// Event send event
func (s *Service) Event(_ *common.SendEvent) bool {
	return true
}

func (s *Service) OnFinish() {}
