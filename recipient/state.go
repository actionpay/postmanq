package recipient

import (
	"bytes"
	"net/textproto"
	"regexp"
)

type StateStatus int

const (
	SuccessStatus StateStatus = iota
	FailureStatus
	PossibleStatus
)

type StateError struct {
	code    int
	message string
}

type State interface {
	SetEvent(*Event)
	GetNext() State
	SetNext(State)
	GetPossibles() []State
	SetPossibles([]State)
	Read(*textproto.Conn) StateStatus
	Write(*textproto.Conn)
}

var (
	crlf = []byte("\r\n")
	sp   = []byte(" ")

	hostnameRegex = regexp.MustCompile(`^[\w\d\.\-]+\.\w{2,5}$`)

	ehlo       = []byte("EHLO")
	ehloLen    = len(ehlo)
	helo       = []byte("HELO")
	heloLen    = len(helo)
	mailCmd    = []byte("MAIL FROM:")
	mailCmdLen = len(mailCmd)
)

type BaseState struct {
	event     *Event
	next      State
	possibles []State
	error     *StateError
}

func (b *BaseState) SetEvent(event *Event) {
	b.event = event
}

func (b BaseState) GetNext() State {
	return b.next
}

func (b *BaseState) SetNext(next State) {
	b.next = next
}

func (b BaseState) GetPossibles() []State {
	return b.possibles
}

func (b *BaseState) SetPossibles(possibles []State) {
	b.possibles = possibles
}

type ConnectState struct {
	BaseState
}

func (c *ConnectState) Read(conn *textproto.Conn) StateStatus {
	return SuccessStatus
}

func (c *ConnectState) Write(conn *textproto.Conn) {
	conn.Cmd("220 %s ESMTP", c.event.serverHostname)
}

type EhloState struct {
	BaseState
	useEhlo bool
}

func (e *EhloState) Read(conn *textproto.Conn) StateStatus {
	line, err := conn.ReadLineBytes()
	if err == nil {
		if bytes.Equal(ehlo, line[:ehloLen]) {
			e.useEhlo = true
			if hostnameRegex.Match(line[ehloLen:]) {
				e.event.clientHostname = line[ehloLen:]
			}
		} else if bytes.Equal(helo, line[:heloLen]) {

		} else {
			return FailureStatus
		}
	} else {

		return FailureStatus
	}
}

func (e *EhloState) Write(conn *textproto.Conn) {
	if e.error == nil {
		if e.useEhlo {

		} else {

		}
	} else {
		conn.Cmd(e.error.message, e.error.code)
	}
}

type MailState struct {
	BaseState
}

func (m *MailState) Read(conn *textproto.Conn) StateStatus {
	return nil
}

func (m *MailState) Write(conn *textproto.Conn) {

}
