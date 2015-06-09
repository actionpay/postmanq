package connector

var (
	seekerEvents = make(chan *ConnectionEvent)
)

type Seeker struct {
	id int
}

func newSeeker(id int) *Seeker {
	return &Seeker{id}
}

func (s *Seeker) run() {
	for _, event := range seekerEvents {
		s.seek(event)
	}
}

func (s *Seeker) seek(event *ConnectionEvent) {

}

