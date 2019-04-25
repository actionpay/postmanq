package consumer

import (
	"github.com/Halfi/postmanq/common"
	"strings"
)

var (
	// карта признаков ошибок, используется для распределения неотправленных сообщений по очередям для ошибок
	errorSignsMap = ErrorSignsMap{
		501: ErrorSigns{
			ErrorSign{RecipientFailureBindingType, []string{
				"bad address syntax",
			}},
		},
		502: ErrorSigns{
			ErrorSign{TechnicalFailureBindingType, []string{
				"syntax error",
			}},
		},
		503: ErrorSigns{
			ErrorSign{TechnicalFailureBindingType, []string{
				"sender not yet given",
				"sender already",
				"bad sequence",
				"commands were rejected",
				"rcpt first",
				"rcpt command",
				"mail first",
				"mail command",
				"mail before",
			}},
			ErrorSign{RecipientFailureBindingType, []string{
				"account blocked",
				"user unknown",
			}},
		},
		504: ErrorSigns{
			ErrorSign{RecipientFailureBindingType, []string{
				"mailbox is disabled",
			}},
		},
		511: ErrorSigns{
			ErrorSign{RecipientFailureBindingType, []string{
				"can't lookup",
			}},
		},
		540: ErrorSigns{
			ErrorSign{RecipientFailureBindingType, []string{
				"recipient address rejected",
				"account has been suspended",
				"account deleted",
			}},
		},
		550: ErrorSigns{
			ErrorSign{TechnicalFailureBindingType, []string{
				"sender verify failed",
				"callout verification failed:",
				"relay",
				"verification failed",
				"unnecessary spaces",
				"host lookup failed",
				"client host rejected",
				"backresolv",
				"can't resolve hostname",
				"reverse",
				"authentication required",
				"bad commands",
				"double-checking",
				"system has detected that",
				"more information",
				"message has been blocked",
				"unsolicited mail",
				"blacklist",
				"black list",
				"not allowed to send",
				"dns operator",
			}},
			ErrorSign{RecipientFailureBindingType, []string{
				"unknown",
				"no such",
				"not exist",
				"disabled",
				"invalid mailbox",
				"not found",
				"mailbox unavailable",
				"has been suspended",
				"inactive",
				"account unavailable",
				"addresses failed",
				"mailbox is frozen",
				"address rejected",
				"administrative prohibition",
				"cannot deliver",
				"unrouteable address",
				"user banned",
				"policy rejection",
				"verify recipient",
				"mailbox locked",
				"blocked",
				"no mailbox",
				"bad destination mailbox",
				"not stored this user",
				"homo hominus",
			}},
			ErrorSign{ConnectionFailureBindingType, []string{
				"spam",
				"is full",
				"over quota",
				"quota exceeded",
				"message rejected",
				"was not accepted",
				"content denied",
				"timeout",
				"support.google.com",
			}},
		},
		552: ErrorSigns{
			ErrorSign{ConnectionFailureBindingType, []string{
				"receiving disabled",
				"is full",
				"over quot",
				"to big",
			}},
		},
		553: ErrorSigns{
			ErrorSign{RecipientFailureBindingType, []string{
				"list of allowed",
				"ecipient has been denied",
			}},
			ErrorSign{TechnicalFailureBindingType, []string{
				"relay",
			}},
			ErrorSign{ConnectionFailureBindingType, []string{
				"does not accept mail from",
			}},
		},
		554: ErrorSigns{
			ErrorSign{TechnicalFailureBindingType, []string{
				"relay access denied",
				"unresolvable address",
				"blocked using",
			}},
			ErrorSign{RecipientFailureBindingType, []string{
				"recipient address rejected",
				"user doesn't have",
				"no such user",
				"inactive user",
				"user unknown",
				"has been disabled",
				"should log in",
				"no mailbox here",
			}},
			ErrorSign{ConnectionFailureBindingType, []string{
				"spam message rejected",
				"suspicion of spam",
				"synchronization error",
				"refused",
			}},
		},
		571: ErrorSigns{
			ErrorSign{ConnectionFailureBindingType, []string{
				"relay",
			}},
		},
		578: ErrorSigns{
			ErrorSign{ConnectionFailureBindingType, []string{
				"address rejected with reverse-check",
			}},
		},
	}
)

// карта признаков ошибок, в качестве ключа используется код ошибки, полученной от почтового сервиса
type ErrorSignsMap map[int]ErrorSigns

// отдает идентификатор очереди, в которую необходимо положить письмо с ошибкой
func (e ErrorSignsMap) BindingType(message *common.MailMessage) FailureBindingType {
	if signs, ok := e[message.Error.Code]; ok {
		return signs.BindingType(message)
	} else {
		return UnknownFailureBindingType
	}
}

// признаки ошибок
type ErrorSigns []ErrorSign

// отдает идентификатор очереди, в которую необходимо положить письмо с ошибкой
func (e ErrorSigns) BindingType(message *common.MailMessage) FailureBindingType {
	bindingType := UnknownFailureBindingType
	for _, sign := range e {
		if sign.resemble(message) {
			bindingType = sign.bindingType
			break
		}
	}
	return bindingType
}

// признак ошибки
type ErrorSign struct {
	// идентификатор очереди
	bindingType FailureBindingType

	// возможные части сообщения, по которым ошибка соотносится с очередью для ошибок
	parts []string
}

// ищет возможные части сообщения в сообщении ошибки
func (e ErrorSign) resemble(message *common.MailMessage) bool {
	hasPart := false
	for _, part := range e.parts {
		if strings.Contains(message.Error.Message, part) {
			hasPart = true
			break
		}
	}
	return hasPart
}
