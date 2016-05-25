package recipient

import (
	"github.com/actionpay/postmanq/logger"
	"net"
	"net/textproto"
)

type Recipient struct {
	id    int
	first State
	state State
	conn  net.Conn
}

func newRecipient(id int, events chan *Event) {
	quit := new(QuitState)
	noop := new(NoopState)
	rset := new(RsetState)
	vrfy := new(VrfyState)

	commonPossibles := []State{
		quit,
		noop,
		rset,
		vrfy,
	}

	input := new(InputState)
	input.SetPossibles(commonPossibles)

	data := new(DataState)
	data.SetNext(input)
	data.SetPossibles(commonPossibles)

	rcpt := new(RcptState)
	rcpt.SetNext(data)
	rcpt.SetPossibles(commonPossibles)

	mail := new(MailState)
	mail.SetNext(rcpt)
	mail.SetPossibles(commonPossibles)

	input.SetNext(mail)
	rset.SetNext(mail)

	ehlo := new(EhloState)
	ehlo.SetNext(mail)
	ehlo.SetPossibles(commonPossibles)

	conn := new(ConnectState)
	conn.SetNext(ehlo)
	conn.SetPossibles(commonPossibles)

	recipient := &Recipient{
		id:    id,
		first: conn,
	}
	for event := range events {
		recipient.handle(event)
	}
}

func (r *Recipient) handle(event *Event) {
	var id uint
	var buf []byte
	txt := textproto.NewConn(event.conn)
	//status := ReadStatus
	r.state = r.first

	statuses := make(StateStatuses)
	statuses.Add(ReadStatus)
	for status := range statuses {
		r.state.SetEvent(event)

		switch status {
		case ReadStatus:
			id = txt.Next()
			txt.StartRequest(id)
			buf = r.state.Read(txt)
			txt.EndRequest(id)
			cmd, cmdLen := r.state.GetCmd()
			if r.state.Check(buf, cmd, cmdLen) {
				statuses.Add(r.state.Process(buf))
			} else {
				statuses.Add(PossibleStatus)
			}
			logger.By(event.serverHostname).Debug(string(buf))

		case WriteStatus:
			txt.StartResponse(id)
			r.state.Write(txt)
			txt.EndResponse(id)

			r.state = r.state.GetNext()
			statuses.Add(ReadStatus)

		case PossibleStatus:
			var possibleStatus StateStatus
			var state State
			for _, possible := range r.state.GetPossibles() {
				possible.SetEvent(event)
				cmd, cmdLen := possible.GetCmd()
				if possible.Check(buf, cmd, cmdLen) {
					possibleStatus = possible.Process(buf)
					if possibleStatus != FailureStatus {
						state = possible
						status = possibleStatus
						break
					}
				}
			}
			if state == nil {
				state := r.first
				for state.GetNext() != nil {

				}
				txt.PrintfLine(syntaxErrorResp)
			} else {
				if state.IsUseCurrent() {
					state.SetNext(r.state)
				}
				r.state = state
			}

		case FailureStatus:
			txt.StartResponse(id)
			txt.PrintfLine(r.state.GetError().message)
			txt.EndResponse(id)
			logger.By(event.serverHostname).Debug("%s: %s", buf, r.state.GetError().message)
			//statuses.Add(ReadStatus)

		case QuitStatus:
			txt.StartResponse(id)
			r.state.Write(txt)
			txt.EndResponse(id)
			txt.Close()
			return
		}
	}
	close(statuses)
}
