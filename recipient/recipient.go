package recipient

import (
	"github.com/actionpay/postmanq/logger"
	"net"
	"net/textproto"
)

type Recipient struct {
	id    int
	state State
	conn  net.Conn
	txt   *textproto.Conn
}

func newRecipient(id int, events chan *Event) {
	data := new(DataState)

	rcpt := new(RcptState)
	rcpt.SetNext(data)

	mail := new(MailState)
	mail.SetNext(rcpt)
	mail.SetPossibles([]State{})

	ehlo := new(EhloState)
	ehlo.SetNext(mail)
	ehlo.SetPossibles([]State{})

	conn := new(ConnectState)
	conn.SetNext(ehlo)
	conn.SetPossibles([]State{})

	recipient := &Recipient{
		id:    id,
		state: conn,
	}
	for event := range events {
		recipient.handle(event)
	}
}

func (r *Recipient) handle(event *Event) {
	r.txt = textproto.NewConn(event.conn)

	for {
		r.state.SetEvent(event)
		status := r.state.Read(r.txt)
		goto handleStatus

	handleStatus:
		switch status {
		case SuccessStatus:
			r.state.Write(r.txt)
			state := r.state.GetNext()
			//if state != nil {
			state.SetId(r.state.GetId())
			r.state = state
			//}

		case WaitingStatus:
			return
		case FailureStatus:
			r.txt.Cmd("500 Syntax error, command unrecognized")
			//r.state.Write(r.txt)

		case PossibleStatus:
			var possibleStatus StateStatus
			for _, possible := range r.state.GetPossibles() {
				possible.SetEvent(event)
				possibleStatus = possible.Read(r.txt)
				if possibleStatus == SuccessStatus {
					possible.SetId(r.state.GetId())
					r.state = possible
					status = possibleStatus
					goto handleStatus
				}
			}
			r.txt.Cmd("500 Syntax error, command unrecognized")

		}

		logger.By("localhost").Info("%v", event.message)
	}
}

func (r *Recipient) handleRequest(event *Event, state State, i int) {
	logger.By("localhost").Info("%T read state id %d", state, state.GetId())
	state.SetEvent(event)
	status := state.Read(r.txt)
	goto handleStatus

handleStatus:
	logger.By("localhost").Info("%T status %v", state, status)
	switch status {
	case SuccessStatus:
		state.Write(r.txt)
		nextState := state.GetNext()
		nextState.SetId(state.GetId())
		r.state = nextState
		logger.By("localhost").Info("%T write state id %d", state, state.GetId())

	case FailureStatus:
		state.Write(r.txt)

	case PossibleStatus:
		var possibleStatus StateStatus
		for _, possible := range state.GetPossibles() {
			possible.SetEvent(event)
			possibleStatus = possible.Read(r.txt)
			if possibleStatus == SuccessStatus {
				possible.SetId(state.GetId())
				r.state = possible
				status = possibleStatus
				goto handleStatus
			}
		}
		r.txt.Cmd("500 Syntax error, command unrecognized")

	}
	return
}
