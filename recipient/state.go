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
	FailureStatus StateStatus = iota + 1
	ReadStatus
	WriteStatus
	PossibleStatus
	QuitStatus
)

type StateError struct {
	message string
}

type State interface {
	SetEvent(*Event)
	GetNext() State
	SetNext(State)
	GetPossibles() []State
	SetPossibles([]State)
	Read(*textproto.Conn) []byte
	Process([]byte) StateStatus
	Write(*textproto.Conn)
	GetId() uint
	SetId(uint)
	IsUseCurrent() bool
}

var (
	crlf           = "\r\n"
	greetResp      = fmt.Sprintf("%d %s", ReadyCode, "%s ESMTP")
	ehloExtensions = []string{
		fmt.Sprintf("%d-STARTTLS", CompleteCode),
		fmt.Sprintf("%d-SIZE", CompleteCode),
		fmt.Sprintf("%d HELP", CompleteCode),
	}
	ehloResp        = fmt.Sprintf("%d-%s", CompleteCode, "%s ready to serve") + crlf + strings.Join(ehloExtensions, crlf)
	heloResp        = fmt.Sprintf("%d %s", CompleteCode, "%s ready to serve")
	completeResp    = fmt.Sprintf("%d OK", CompleteCode)
	startInputResp  = fmt.Sprintf(StartInputCode.GetName(), StartInputCode)
	closeResp       = CloseCode.GetName()
	syntaxErrorResp = SyntaxErrorCode.GetName()

	ehlo       = []byte("EHLO")
	ehloLen    = len(ehlo)
	helo       = []byte("HELO")
	heloLen    = len(helo)
	mailCmd    = []byte("MAIL FROM:")
	mailCmdLen = len(mailCmd)
	rcptCmd    = []byte("RCPT TO:")
	rcptCmdLen = len(rcptCmd)
	dataCmd    = []byte("DATA")
	dataCmdLen = len(dataCmd)
	quitCmd    = []byte("QUIT")
	quitCmdLen = len(quitCmd)
	noopCmd    = []byte("NOOP")
	noopCmdLen = len(quitCmd)
	rsetCmd    = []byte("RSET")
	rsetCmdLen = len(quitCmd)
	vrfyCmd    = []byte("VRFY")
	vrfyCmdLen = len(quitCmd)
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
	conn.PrintfLine(b.error.message)
}

func (b BaseState) checkCmd(line []byte, cmd []byte, cmdLen int) bool {
	return len(line) >= cmdLen && bytes.Equal(cmd, bytes.ToUpper(line[:cmdLen]))
}

func (b BaseState) Read(conn *textproto.Conn) []byte {
	line, err := conn.ReadLineBytes()
	if err == nil {
		return line
	} else {
		return nil
	}
}

func (b BaseState) IsUseCurrent() bool {
	return false
}

type ConnectState struct {
	BaseState
}

func (c *ConnectState) Read(conn *textproto.Conn) []byte {
	return nil
}

func (c *ConnectState) Process(line []byte) StateStatus {
	return WriteStatus
}

func (c *ConnectState) Write(conn *textproto.Conn) {
	conn.PrintfLine(greetResp, c.event.serverHostname)
	logger.By(c.event.serverHostname).Debug(greetResp, c.event.serverHostname)
}

type EhloState struct {
	BaseState
	useEhlo bool
}

func (e *EhloState) Process(line []byte) StateStatus {
	status := e.receiveClientHostname(line, ehlo, ehloLen)
	if status == WriteStatus {
		e.useEhlo = true
	} else {
		status = e.receiveClientHostname(line, helo, heloLen)
	}
	return status
}

func (e *EhloState) receiveClientHostname(line []byte, cmd []byte, cmdLen int) StateStatus {
	if e.checkCmd(line, cmd, cmdLen) {
		hostname := bytes.TrimSpace(line[cmdLen:])
		if common.HostnameRegex.Match(hostname) {
			e.event.clientHostname = hostname
			return WriteStatus
		} else {
			e.error = &StateError{
				message: SyntaxParamErrorCode.GetFormattedName(),
			}
			return FailureStatus
		}
	}
	return PossibleStatus
}

func (e *EhloState) Write(conn *textproto.Conn) {
	var resp string

	if e.useEhlo {
		resp = ehloResp
	} else {
		resp = heloResp
	}

	conn.PrintfLine(resp, e.event.serverMxHostname)
	logger.By(e.event.serverHostname).Debug(resp, e.event.serverMxHostname)
}

type MailState struct {
	BaseState
}

func (m *MailState) Process(line []byte) StateStatus {
	if m.checkCmd(line, mailCmd, mailCmdLen) {
		envelope := line[mailCmdLen+1 : len(line)-1]
		if common.EmailRegexp.Match(envelope) {
			m.event.message = &common.MailMessage{
				Envelope: string(envelope),
			}
			return WriteStatus
		} else {

			return FailureStatus
		}
	}
	return PossibleStatus

}

func (m *MailState) Write(conn *textproto.Conn) {
	conn.PrintfLine(completeResp)
	logger.By(m.event.serverHostname).Debug(completeResp)
}

type RcptState struct {
	BaseState
}

func (r *RcptState) Process(line []byte) StateStatus {
	if r.checkCmd(line, rcptCmd, rcptCmdLen) {
		recipient := line[rcptCmdLen+1 : len(line)-1]
		if common.EmailRegexp.Match(recipient) {
			r.event.message.Recipient = string(recipient)
			return WriteStatus
		} else {
			logger.By(r.event.serverHostname).Warn(completeResp)
			return FailureStatus
		}
	}

	return PossibleStatus
}

func (r *RcptState) Write(conn *textproto.Conn) {
	conn.PrintfLine(completeResp)
	logger.By(r.event.serverHostname).Debug(completeResp)
}

type DataState struct {
	BaseState
}

func (d *DataState) Process(line []byte) StateStatus {
	if d.checkCmd(line, dataCmd, dataCmdLen) {
		return WriteStatus
	}
	return FailureStatus
}

func (d *DataState) Write(conn *textproto.Conn) {
	conn.PrintfLine(startInputResp)
	logger.By(d.event.serverHostname).Debug(startInputResp)
}

type InputState struct {
	BaseState
}

func (i *InputState) Read(conn *textproto.Conn) []byte {
	line, err := conn.ReadDotBytes()
	if err == nil {
		return line
	} else {
		return nil
	}
}

func (i *InputState) Process(line []byte) StateStatus {
	i.event.message.Body = string(line)
	logger.By(i.event.serverHostname).Debug(
		"envelope: %s, recipient: %s, body: %s",
		i.event.message.Envelope,
		i.event.message.Recipient,
		i.event.message.Body,
	)
	return WriteStatus
}

func (i *InputState) Write(conn *textproto.Conn) {
	conn.PrintfLine(completeResp)
	logger.By(i.event.serverHostname).Debug(completeResp)
}

type QuitState struct {
	BaseState
}

func (q *QuitState) Process(line []byte) StateStatus {
	if q.checkCmd(line, quitCmd, quitCmdLen) {
		return QuitStatus
	}
	return FailureStatus
}

func (q *QuitState) Write(conn *textproto.Conn) {
	conn.PrintfLine(closeResp, CloseCode, q.event.serverMxHostname)
	logger.By(q.event.serverHostname).Debug(closeResp, CloseCode, q.event.serverMxHostname)
	conn.Close()
}

type NoopState struct {
	BaseState
}

func (n *NoopState) Process(line []byte) StateStatus {
	if n.checkCmd(line, noopCmd, noopCmdLen) {
		return WriteStatus
	}
	return FailureStatus
}

func (n *NoopState) Write(conn *textproto.Conn) {
	conn.PrintfLine(completeResp)
	logger.By(n.event.serverHostname).Debug(completeResp)
}

func (n NoopState) IsUseCurrent() bool {
	return true
}

type RsetState struct {
	BaseState
}

func (r *RsetState) Process(line []byte) StateStatus {
	if r.checkCmd(line, rsetCmd, rsetCmdLen) {
		return WriteStatus
	}
	return FailureStatus
}

func (r *RsetState) Write(conn *textproto.Conn) {
	r.event.message = nil
	conn.PrintfLine(completeResp)
	logger.By(r.event.serverHostname).Debug(completeResp)
}

type VrfyState struct {
	BaseState
}

func (v *VrfyState) Process(line []byte) StateStatus {
	if v.checkCmd(line, vrfyCmd, vrfyCmdLen) {
		if common.EmailRegexp.Match(line[vrfyCmdLen+1:]) {
			return WriteStatus
		} else {
			return FailureStatus
		}
	}
	return FailureStatus
}

func (v *VrfyState) Write(conn *textproto.Conn) {
	conn.PrintfLine(completeResp)
	logger.By(v.event.serverHostname).Debug(completeResp)
}

func (v VrfyState) IsUseCurrent() bool {
	return true
}
