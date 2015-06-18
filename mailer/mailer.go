package mailer

import (
	"errors"
	"fmt"
	"github.com/AdOnWeb/postmanq/common"
	"github.com/AdOnWeb/postmanq/logger"
	"github.com/byorty/dkim"
)

type Mailer struct {
	id int
}

func newMailer(id int) *Mailer {
	return &Mailer{id}.run()
}

func (m *Mailer) run() {
	for event := range events {
		m.sendMail(event)
	}
}

func (m *Mailer) sendMail(event *common.SendEvent) {
	message := event.Message
	if common.EmailRegexp.MatchString(message.Envelope) && common.EmailRegexp.MatchString(message.Recipient) {
		m.prepare(message)
		m.send(event)
	} else {
		ReturnMail(event, errors.New(fmt.Sprintf("511 service#%d can't send mail#%d, envelope or ricipient is invalid", m.id, message.Id)))
	}
}

func (m *Mailer) prepare(message *common.MailMessage) {
	conf, err := dkim.NewConf(message.HostnameFrom, service.DkimSelector)
	if err == nil {
		conf[dkim.AUIDKey] = message.Envelope
		conf[dkim.CanonicalizationKey] = "relaxed/relaxed"
		signer, err := dkim.New(conf, service.privateKey)
		if err == nil {
			signed, err := signer.Sign([]byte(message.Body))
			if err == nil {
				message.Body = string(signed)
			} else {
				logger.Warn("service#%d can't sign mail#%d, error - %v", m.id, message.Id, err)
			}
		} else {
			logger.Warn("service#%d can't create dkim for mail#%d, error - %v", m.id, message.Id, err)
		}
	} else {
		logger.Warn("service#%d can't create dkim config for mail#%d, error - %v", m.id, message.Id, err)
	}
}

func (m *Mailer) send(event *common.SendEvent) {
	message := event.Message
	worker := event.Client.Worker
	logger.Info("service#%d try send mail#%d", m.id, message.Id)
	logger.Debug("service#%d receive smtp client#%d", m.id, event.Client.Id)

	success := false
	err := worker.Mail(message.Envelope)
	if err == nil {
		logger.Debug("service#%d send command MAIL FROM: %s", m.id, message.Envelope)
		event.Client.SetTimeout(RcptTimeout)
		err = worker.Rcpt(message.Recipient)
		if err == nil {
			logger.Debug("service#%d send command RCPT TO: %s", m.id, message.Recipient)
			event.Client.SetTimeout(DataTimeout)
			wc, err := worker.Data()
			if err == nil {
				logger.Debug("service#%d send command DATA", m.id)
				_, err = fmt.Fprint(wc, message.Body)
				if err == nil {
					wc.Close()
					logger.Debug("%s", message.Body)
					logger.Debug("service#%d send command .", m.id)
					// стараемся слать письма через уже созданное соединение,
					// поэтому после отправки письма не закрываем соединение
					err = worker.Reset()
					if err == nil {
						logger.Debug("service#%d send command RSET", m.id)
						logger.Info("service#%d success send mail#%d", m.id, message.Id)
						success = true
					}
				}
			}
		}
	}

	event.Client.Wait()
	event.Queue.Push(event.Client)

	if success {
		// отпускаем поток получателя сообщений из очереди
		event.Result <- SuccessSendEventResult
	} else {
		common.ReturnMail(event, err)
	}
}
