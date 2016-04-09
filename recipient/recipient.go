package recipient

import (
	"net"
	"net/textproto"
)

//type RecipientState int
//
//const (
//	Ehlo RecipientState = iota
//	Helo
//	Mail
//	Rcpt
//	Data
//	Rset
//	Noop
//	Vrfy
//	Quit
//)
//
//var (
//	nexts = map[RecipientState][]RecipientState {
//		Ehlo: []RecipientState{Mail, Rset, Noop, Quit, Vrfy},
//		Helo: []RecipientState{Mail, Rset, Noop, Quit, Vrfy},
//		Mail: []RecipientState{Rcpt, Rset, Noop, Quit},
//		Rcpt: []RecipientState{Data, Rcpt, Rset, Noop, Quit},
//		Data: []RecipientState{Mail, Rset, Noop, Quit},
//		Rset: []RecipientState{Mail, Rset, Noop, Quit},
//		Noop: []RecipientState{},
//		Vrfy: []RecipientState{},
//		Quit: []RecipientState{},
//	}
//)
//
//func (r RecipientState) getName() []byte {}

type Recipient struct {
	id    int
	state State
	conn  net.Conn
	txt   *textproto.Conn
}

func newRecipient(id int, events chan *Event) {
	mail := new(MailState)
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
			r.state = r.state.GetNext()

		case FailureStatus:
			r.state.Write(r.txt)

		case PossibleStatus:
			var possibleStatus StateStatus
			for _, possible := range r.state.GetPossibles() {
				possible.SetEvent(event)
				possibleStatus = possible.Read(r.txt)
				if possibleStatus == SuccessStatus {
					r.state = possible
					status = possibleStatus
					goto handleStatus
				}
			}
			r.txt.Cmd("500 Syntax error, command unrecognized")

		}
		return
	}
}
