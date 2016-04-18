package recipient

import (
	"bytes"
	"fmt"
	"github.com/actionpay/postmanq/common"
	//"github.com/actionpay/postmanq/logger"
	"net/textproto"
	"strings"
)

type StateStatus int

const (
	SuccessStatus StateStatus = iota + 1
	FailureStatus
	ReadStatus
	WriteStatus
	PossibleStatus
	QuitStatus
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
	Read(*textproto.Conn) []byte
	Check([]byte) bool
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
	ehloResp       = fmt.Sprintf("%d-%s", CompleteCode, "%s ready to serve") + crlf + strings.Join(ehloExtensions, crlf)
	heloResp       = fmt.Sprintf("%d %s", CompleteCode, "%s ready to serve")
	completeResp   = fmt.Sprintf("%d OK", CompleteCode)
	startInputResp = fmt.Sprintf(StartInputCode.GetName(), StartInputCode)
	closeResp      = CloseCode.GetName()

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
	return len(line) >= cmdLen && bytes.Equal(cmd, bytes.ToUpper(line[:cmdLen]))
}

type ConnectState struct {
	BaseState
}

func (c *ConnectState) Read(conn *textproto.Conn) []byte {
	return nil
}

func (c *ConnectState) Check(line []byte) bool {
	return true
}

func (c *ConnectState) Write(conn *textproto.Conn) {
	conn.PrintfLine(greet, c.event.serverHostname)
	fmt.Printf(greet, c.event.serverHostname)
	fmt.Println()
}

type EhloState struct {
	BaseState
	useEhlo bool
}

func (e *EhloState) Read(conn *textproto.Conn) []byte {
	//conn.StartRequest(e.id)
	//defer conn.EndRequest(e.id)
	line, err := conn.ReadLineBytes()
	//logger.By("localhost").Info(string(line))
	fmt.Printf(string(line))
	fmt.Println()
	if err == nil {
		//status := e.receiveClientHostname(line, ehlo, ehloLen)
		//if status == FailureStatus {
		//	return line, e.receiveClientHostname(line, helo, heloLen)
		//}
		//e.useEhlo = true
		return line
	} else {
		return nil
	}
}

func (e *EhloState) Check(line []byte) bool {
	return e.checkCmd(line, ehlo, ehloLen)
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

	//e.id = conn.Next()
	//conn.StartResponse(e.id)
	//defer conn.EndResponse(e.id)
	conn.PrintfLine(resp, e.event.serverMxHostname)
	//logger.By("localhost").Info(resp, e.event.serverMxHostname)
	fmt.Printf(resp, e.event.serverMxHostname)
	fmt.Println()
}

//type MailState struct {
//	BaseState
//}
//
//func (m *MailState) Read(conn *textproto.Conn) StateStatus {
//
//	conn.StartRequest(m.id)
//	defer conn.EndRequest(m.id)
//	line, err := conn.ReadLineBytes()
//	//logger.By("localhost").Info(string(line))
//	fmt.Printf(string(line))
//	fmt.Println()
//	if err == nil {
//		if m.checkCmd(line, mailCmd, mailCmdLen) {
//			envelope := line[mailCmdLen+1 : len(line)-1]
//			if common.EmailRegexp.Match(envelope) {
//				m.event.message = &common.MailMessage{
//					Envelope: string(envelope),
//				}
//				return SuccessStatus
//			} else {
//				m.error = &StateError{}
//				// error
//			}
//		} else {
//			m.event.possibleCmd = line
//			return PossibleStatus
//		}
//	}
//	return FailureStatus
//}
//
//func (m *MailState) Write(conn *textproto.Conn) {
//	m.id = conn.Next()
//	conn.StartResponse(m.id)
//	defer conn.EndResponse(m.id)
//	conn.PrintfLine(completeResp)
//	//logger.By("localhost").Info(completeResp)
//	fmt.Printf(completeResp)
//	fmt.Println()
//}

//type RcptState struct {
//	BaseState
//}
//
//func (r *RcptState) Read(conn *textproto.Conn) StateStatus {
//
//	conn.StartRequest(r.id)
//	defer conn.EndRequest(r.id)
//	line, err := conn.ReadLineBytes()
//	//logger.By("localhost").Info(string(line))
//	fmt.Printf(string(line))
//	fmt.Println()
//	if err == nil {
//		if r.checkCmd(line, rcptCmd, rcptCmdLen) {
//			recipient := line[rcptCmdLen+1 : len(line)-1]
//			if common.EmailRegexp.Match(recipient) {
//				r.event.message.Recipient = string(recipient)
//				return SuccessStatus
//			} else {
//				// error
//			}
//		}
//	}
//	return FailureStatus
//}
//
//func (r *RcptState) Write(conn *textproto.Conn) {
//	r.id = conn.Next()
//	conn.StartResponse(r.id)
//	defer conn.EndResponse(r.id)
//	conn.PrintfLine(completeResp)
//	//logger.By("localhost").Info(completeResp)
//	fmt.Printf(completeResp)
//	fmt.Println()
//	//r.id = conn.Next()
//}
//
//type DataState struct {
//	BaseState
//}
//
//func (d *DataState) Read(conn *textproto.Conn) StateStatus {
//
//	conn.StartRequest(d.id)
//	defer conn.EndRequest(d.id)
//	line, err := conn.ReadLineBytes()
//	//logger.By("localhost").Info(string(line))
//	fmt.Printf(string(line))
//	fmt.Println()
//	if err == nil {
//		if d.checkCmd(line, dataCmd, dataCmdLen) {
//			return SuccessStatus
//		}
//	}
//	return FailureStatus
//}
//
//func (d *DataState) Write(conn *textproto.Conn) {
//
//	//d.id, _ = conn.Cmd(startInputResp)
//	//logger.By("localhost").Info(startInputResp)
//	fmt.Printf(startInputResp)
//	fmt.Println()
//
//	d.id = conn.Next()
//	conn.StartResponse(d.id)
//	defer conn.EndResponse(d.id)
//	conn.PrintfLine(startInputResp)
//	//line, _ := conn.ReadDotBytes()
//	//fmt.Printf(string(line))
//	//fmt.Println()
//
//	//conn.StartResponse(d.id)
//	//defer conn.EndResponse(d.id)
//}
//
//type InputState struct {
//	BaseState
//}
//
//func (i *InputState) Read(conn *textproto.Conn) StateStatus {
//	conn.StartRequest(i.id)
//	defer conn.EndRequest(i.id)
//	line, err := conn.ReadDotBytes()
//	str := string(line)
//	fmt.Printf(str)
//	fmt.Println()
//	if err == nil {
//		i.event.message.Body = str
//		return SuccessStatus
//	}
//	return FailureStatus
//}
//
//func (i *InputState) Write(conn *textproto.Conn) {
//	i.id = conn.Next()
//	conn.StartResponse(i.id)
//	defer conn.EndResponse(i.id)
//	conn.PrintfLine(completeResp)
//	//i.id, _ = conn.Cmd(completeResp)
//	//logger.By("localhost").Info(completeResp)
//	fmt.Printf(completeResp)
//	fmt.Println()
//}
//
//type QuitState struct {
//	BaseState
//}
//
//func (q *QuitState) Read(conn *textproto.Conn) StateStatus {
//	//conn.StartRequest(q.id)
//	//defer conn.EndRequest(q.id)
//	//line, err := conn.ReadLineBytes()
//
//	//if err == nil {
//	fmt.Printf(string(q.event.possibleCmd))
//	fmt.Println()
//	fmt.Println(bytes.Equal(q.event.possibleCmd, bytes.ToUpper(q.event.possibleCmd[:quitCmdLen])))
//	fmt.Println()
//		if q.checkCmd(q.event.possibleCmd, quitCmd, quitCmdLen) {
//			return QuitStatus
//		}
//	//}
//	return FailureStatus
//
//}
//
//func (q *QuitState) Write(conn *textproto.Conn) {
//	q.id = conn.Next()
//	conn.StartResponse(q.id)
//	defer conn.EndResponse(q.id)
//	conn.PrintfLine(closeResp, CloseCode, q.event.serverMxHostname)
//	fmt.Printf(closeResp, CloseCode, q.event.serverMxHostname)
//	fmt.Println()
//}
