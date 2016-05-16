package recipient

import "fmt"

type Code int

func (c Code) GetName() string {
	return codeMessages[c]
}

func (c Code) GetFormattedName() string {
	return fmt.Sprintf(c.GetName(), c)
}

const (
	StatusCode              Code = 211
	HelpCode                Code = 214
	ReadyCode               Code = 220
	CloseCode               Code = 221
	CompleteCode            Code = 250
	ForwardCode             Code = 251
	AttemptDeliveryCode     Code = 252
	StartInputCode          Code = 354
	NotAvailableCode        Code = 421
	MailboxUnavailableCode  Code = 450
	AbortedCode             Code = 451
	NotTakenCode            Code = 452
	UnableAcceptParamsCode  Code = 455
	SyntaxErrorCode         Code = 500
	SyntaxParamErrorCode    Code = 501
	NotImplementedCode      Code = 502
	BadSequenceCode         Code = 503
	ParamNotImplementedCode Code = 504
	UserNotFoundCode        Code = 550
	UserNotLocalCode        Code = 551
	ExceededStorageCode     Code = 552
	NameNotAllowedCode      Code = 553
	TransactionFailedCode   Code = 554
	ParamsNotRecognizedCode Code = 555
)

var (
	codeMessages = map[Code]string{
		StatusCode:              "%d System status or system help reply",
		HelpCode:                "%d Help message",
		ReadyCode:               "%d %s Service ready",
		CloseCode:               "%d %s Service closing transmission channel",
		CompleteCode:            "%d Requested mail action okay, completed",
		ForwardCode:             "%d User not local",
		AttemptDeliveryCode:     "%d Cannot VRFY user, but will accept message and attempt delivery",
		StartInputCode:          "%d Start mail input",
		NotAvailableCode:        "%d %s Service not available, closing transmission channel",
		MailboxUnavailableCode:  "%d Requested mail action not taken: mailbox unavailable",
		AbortedCode:             "%d Requested action aborted: error in processing",
		NotTakenCode:            "%d Requested action not taken: insufficient system storage",
		UnableAcceptParamsCode:  "%d Server unable to accommodate parameters",
		SyntaxErrorCode:         "%d Syntax error, command unrecognized",
		SyntaxParamErrorCode:    "%d Syntax error in parameters or arguments",
		NotImplementedCode:      "%d Command not implemented",
		BadSequenceCode:         "%d Bad sequence of commands",
		ParamNotImplementedCode: "%d Command parameter not implemented",
		UserNotFoundCode:        "%d Requested action not taken: mailbox unavailable",
		UserNotLocalCode:        "%d User not local",
		ExceededStorageCode:     "%d Requested mail action aborted: exceeded storage allocation",
		NameNotAllowedCode:      "%d Requested action not taken: mailbox name not allowed",
		TransactionFailedCode:   "%d Transaction failed",
		ParamsNotRecognizedCode: "%d MAIL FROM/RCPT TO parameters not recognized or not implemented",
	}
)
