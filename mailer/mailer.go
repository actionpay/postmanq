package mailer

import (
	"errors"
	"fmt"
	"github.com/Halfi/postmanq/common"
	"github.com/Halfi/postmanq/logger"
	"github.com/byorty/dkim"
)

// отправитель письма
type Mailer struct {
	// идентификатор для логов
	id int
}

// создает нового отправителя
func newMailer(id int) {
	mailer := &Mailer{id}
	mailer.run()
}

// запускает отправителя
func (m *Mailer) run() {
	for event := range events {
		m.sendMail(event)
	}
}

// подписывает dkim и отправляет письмо
func (m *Mailer) sendMail(event *common.SendEvent) {
	message := event.Message
	if common.EmailRegexp.MatchString(message.Envelope) && common.EmailRegexp.MatchString(message.Recipient) {
		m.prepare(message)
		m.send(event)
	} else {
		common.ReturnMail(event, errors.New(fmt.Sprintf("511 service#%d can't send mail#%d, envelope or ricipient is invalid", m.id, message.Id)))
	}
}

// подписывает dkim
func (m *Mailer) prepare(message *common.MailMessage) {
	conf, err := dkim.NewConf(message.HostnameFrom, service.getDkimSelector(message.HostnameFrom))
	if err == nil {
		conf[dkim.AUIDKey] = message.Envelope
		conf[dkim.CanonicalizationKey] = "relaxed/relaxed"
		signer := dkim.NewByKey(conf, service.getPrivateKey(message.HostnameFrom))
		if err == nil {
			signed, err := signer.Sign([]byte(message.Body))
			if err == nil {
				message.Body = string(signed)
				logger.By(message.HostnameFrom).Debug("mailer#%d-%d success sign mail", m.id, message.Id)
			} else {
				logger.By(message.HostnameFrom).Warn("mailer#%d-%d can't sign mail, error - %v", m.id, message.Id, err)
			}
		} else {
			logger.By(message.HostnameFrom).Warn("mailer#%d-%d can't create dkim signer, error - %v", m.id, message.Id, err)
		}
	} else {
		logger.By(message.HostnameFrom).Warn("mailer#%d-%d can't create dkim config, error - %v", m.id, message.Id, err)
	}
}

// отправляет письмо
func (m *Mailer) send(event *common.SendEvent) {
	message := event.Message
	worker := event.Client.Worker
	logger.By(event.Message.HostnameFrom).Info("mailer#%d-%d begin sending mail", m.id, message.Id)
	logger.By(message.HostnameFrom).Debug("mailer#%d-%d receive smtp client#%d", m.id, message.Id, event.Client.Id)

	success := false
	event.Client.SetTimeout(common.App.Timeout().Mail)
	err := worker.Mail(message.Envelope)
	if err == nil {
		logger.By(message.HostnameFrom).Debug("mailer#%d-%d send command MAIL FROM: %s", m.id, message.Id, message.Envelope)
		event.Client.SetTimeout(common.App.Timeout().Rcpt)
		err = worker.Rcpt(message.Recipient)
		if err == nil {
			logger.By(message.HostnameFrom).Debug("mailer#%d-%d send command RCPT TO: %s", m.id, message.Id, message.Recipient)
			event.Client.SetTimeout(common.App.Timeout().Data)
			wc, err := worker.Data()
			if err == nil {
				logger.By(message.HostnameFrom).Debug("mailer#%d-%d send command DATA", m.id, message.Id)
				_, err = fmt.Fprint(wc, message.Body)
				if err == nil {
					wc.Close()
					logger.By(message.HostnameFrom).Debug("%s", message.Body)
					logger.By(message.HostnameFrom).Debug("mailer#%d-%d send command .", m.id, message.Id)
					// стараемся слать письма через уже созданное соединение,
					// поэтому после отправки письма не закрываем соединение
					err = worker.Reset()
					if err == nil {
						logger.By(message.HostnameFrom).Debug("mailer#%d-%d send command RSET", m.id, message.Id)
						logger.By(event.Message.HostnameFrom).Info("mailer#%d-%d success send mail#%d", m.id, message.Id, message.Id)
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
		event.Result <- common.SuccessSendEventResult
	} else {
		common.ReturnMail(event, err)
	}
}
