package recipient

type StateStatuses chan StateStatus

func (s StateStatuses) Add(status StateStatus) {
	go func() {
		s <- status
	}()
}
