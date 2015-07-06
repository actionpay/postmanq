package guardian

import (
	"github.com/AdOnWeb/postmanq/common"
	"sort"
)

type Guardian struct {
	// Идентификатор для логов
	id int
}

func newGuardian(id int) {
	guardian := &Guardian{id}
	guardian.run()
}

func (g *Guardian) run() {
	for event := range events {
		g.guard(event)
	}
}

func (g *Guardian) guard(event *common.SendEvent) {
	hostnameTo := event.Message.HostnameTo
	i := sort.Search(service.hostnameLen, func(i int) bool {
		return service.Hostnames[i] == hostnameTo
	})
	if i < service.hostnameLen && service.Hostnames[i] == hostnameTo {

	} else {

	}
}
