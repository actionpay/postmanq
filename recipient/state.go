package recipient

import (
	"bytes"
	"fmt"
	"github.com/actionpay/postmanq/common"
	"github.com/actionpay/postmanq/logger"
	"net/textproto"
	"strings"
)

type StateStatus int

const (
	SuccessStatus StateStatus = iota + 1
	FailureStatus
	PossibleStatus
	WaitingStatus
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
	GetId() uint
	SetId(uint)
}

var (
	crlf           = "\r\n"
	greet          = fmt.Sprintf("%d %s", ReadyCode, "%s ESMTP")
	ehloExtensions = []string{
		fmt.Sprintf("%d-STARTTLS", CompleteCode),
		fmt.Sprintf("%d-SIZE", CompleteCode),
		fmt.Sprintf("%d HELP", CompleteCode),
	}
	ehloResp       = fmt.Sprintf("%d %s", CompleteCode, "%s ready to serve") + crlf + strings.Join(ehloExtensions, crlf)
	heloResp       = fmt.Sprintf("%d %s", CompleteCode, "%s ready to serve")
	completeResp   = fmt.Sprintf("%d OK", CompleteCode)
	startInputResp = fmt.Sprintf(StartInputCode.GetName(), StartInputCode)

	ehlo       = []byte("EHLO")
	ehloLen    = len(ehlo)
	helo       = []byte("HELO")
	heloLen    = len(helo)
	mailCmd    = []byte("MAIL FROM:")
	mailCmdLen = len(mailCmd)
	rcptCmd    = []byte("RCPT TO:")
	rcptCmdLen = len(rcptCmd)
)

type BaseState struct {
	event     *Event
	next      State
	possibles []State
	error     *StateError
	id        uint
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

func (b BaseState) GetId() uint {
	return b.id
}

func (b *BaseState) SetId(id uint) {
	b.id = id
}

func (b BaseState) writeError(conn *textproto.Conn) {
	conn.PrintfLine(b.error.message, b.error.code)
}

func (b BaseState) checkCmd(line []byte, cmd []byte, cmdLen int) bool {
	return bytes.Equal(cmd, bytes.ToUpper(line[:cmdLen]))
}

type ConnectState struct {
	BaseState
}

func (c *ConnectState) Read(conn *textproto.Conn) StateStatus {
	return SuccessStatus
}

func (c *ConnectState) Write(conn *textproto.Conn) {
	c.id, _ = conn.Cmd(greet, c.event.serverHostname)
	logger.By("localhost").Info(greet, c.event.serverHostname)
}

type EhloState struct {
	BaseState
	useEhlo bool
}

func (e *EhloState) Read(conn *textproto.Conn) StateStatus {
	conn.StartResponse(e.id)
	defer conn.EndResponse(e.id)
	line, err := conn.ReadLineBytes()
	logger.By("localhost").Info(string(line))
	if err == nil {
		status := e.receiveClientHostname(line, ehlo, ehloLen)
		if status == FailureStatus {
			return e.receiveClientHostname(line, helo, heloLen)
		}
		e.useEhlo = true
		return status
	} else {
		return FailureStatus
	}
}

func (e *EhloState) receiveClientHostname(line []byte, cmd []byte, cmdLen int) StateStatus {
	if e.checkCmd(line, cmd, cmdLen) {
		hostname := bytes.TrimSpace(line[cmdLen:])
		if common.HostnameRegex.Match(hostname) {
			e.event.clientHostname = hostname
		} else {
			// error
		}
		return SuccessStatus
	}
	return FailureStatus
}

func (e *EhloState) Write(conn *textproto.Conn) {
	var resp string

	if e.useEhlo {
		resp = ehloResp
	} else {
		resp = heloResp
	}

	e.id, _ = conn.Cmd(resp, e.event.serverMxHostname)
	logger.By("localhost").Info(resp, e.event.serverMxHostname)
}

type MailState struct {
	BaseState
}

func (m *MailState) Read(conn *textproto.Conn) StateStatus {
	conn.StartResponse(m.id)
	defer conn.EndResponse(m.id)
	line, err := conn.ReadLineBytes()
	logger.By("localhost").Info(string(line))
	if err == nil {
		if m.checkCmd(line, mailCmd, mailCmdLen) {
			envelope := line[mailCmdLen+1 : len(line)-1]
			if common.EmailRegexp.Match(envelope) {
				m.event.message = &common.MailMessage{
					Envelope: string(envelope),
				}
				return SuccessStatus
			} else {
				m.error = &StateError{}
				// error
			}
		}
	}
	return FailureStatus
}

func (m *MailState) Write(conn *textproto.Conn) {
	m.id, _ = conn.Cmd(completeResp)
	logger.By("localhost").Info(completeResp)
}

type RcptState struct {
	BaseState
}

func (r *RcptState) Read(conn *textproto.Conn) StateStatus {
	conn.StartResponse(r.id)
	defer conn.EndResponse(r.id)
	line, err := conn.ReadLineBytes()
	logger.By("localhost").Info(string(line))
	if err == nil {
		if r.checkCmd(line, rcptCmd, rcptCmdLen) {
			recipient := line[rcptCmdLen+1 : len(line)-1]
			if common.EmailRegexp.Match(recipient) {
				r.event.message.Recipient = string(recipient)
				return SuccessStatus
			} else {
				// error
			}
		}
	}
	return FailureStatus
}

func (r *RcptState) Write(conn *textproto.Conn) {
	r.id, _ = conn.Cmd(completeResp)
	logger.By("localhost").Info(completeResp)
}

type DataState struct {
	BaseState
}

func (d *DataState) Read(conn *textproto.Conn) StateStatus {
	return SuccessStatus
}

func (d *DataState) Write(conn *textproto.Conn) {
	d.id, _ = conn.Cmd(startInputResp)
}
