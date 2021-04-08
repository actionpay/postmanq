package mailer

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/byorty/dkim"
	"github.com/sergw3x/postmanq/common"
	"github.com/sergw3x/postmanq/logger"
	"os"
	"path/filepath"
	"strings"
	"text/template"
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
	conf, err := dkim.NewConf(message.HostnameFrom, service.DkimSelector)
	if err == nil {
		conf[dkim.AUIDKey] = message.Envelope
		conf[dkim.CanonicalizationKey] = "relaxed/relaxed"
		signer := dkim.NewByKey(conf, service.privateKey)

		if !strings.Contains(message.Body, "Mime-Version") || !strings.Contains(message.Body, "<html") {
			if message.ContentType == "" {
				message.ContentType = "text/plain"
			}
			from := fmt.Sprintf("From: <%s>\n", message.Envelope)
			if message.EnvelopeName != "" {
				from = fmt.Sprintf("From: %s <%s>\n", message.EnvelopeName, message.Envelope)
			}

			subject := message.Subject
			if message.Subject == "" {
				subject = "Информационное письмо"
			}

			path := filepath.Join("templates", message.TemplateName+".html")
			if message.TemplateName == "" {
				message.TemplateName = "templates/default.html"
			} else if _, err := os.Stat(path); err != nil {
				message.TemplateName = "templates/default.html"
			} else {
				message.TemplateName = path
			}

			t, _ := template.ParseFiles(message.TemplateName)

			var body bytes.Buffer
			mimeHeaders := fmt.Sprintf("Mime-Version: 1.0;\nContent-Type: %s; charset=\"UTF-8\";\n\n", message.ContentType)
			body.Write([]byte(fmt.Sprintf("%sSubject: %s \n%s\n\n", from, subject, mimeHeaders)))

			t.Execute(&body, struct {
				Message string
			}{
				Message: message.Body,
			})
			message.Body = string(body.Bytes())
		}

		if err == nil {
			signed, err := signer.Sign([]byte(message.Body))
			if err == nil {
				message.Body = string(signed)
				logger.Debug("mailer#%d-%d success sign mail", m.id, message.Id)
			} else {
				logger.Warn("mailer#%d-%d can't sign mail, error - %v", m.id, message.Id, err)
			}
		} else {
			logger.Warn("mailer#%d-%d can't create dkim signer, error - %v", m.id, message.Id, err)
		}
	} else {
		logger.Warn("mailer#%d-%d can't create dkim config, error - %v", m.id, message.Id, err)
	}
}

// отправляет письмо
func (m *Mailer) send(event *common.SendEvent) {
	message := event.Message
	worker := event.Client.Worker
	logger.Info("mailer#%d-%d begin sending mail", m.id, message.Id)
	logger.Debug("mailer#%d-%d receive smtp client#%d", m.id, message.Id, event.Client.Id)

	success := false
	event.Client.SetTimeout(common.App.Timeout().Mail)
	err := worker.Mail(message.Envelope)
	if err == nil {
		logger.Debug("mailer#%d-%d send command MAIL FROM: %s", m.id, message.Id, message.Envelope)
		event.Client.SetTimeout(common.App.Timeout().Rcpt)
		err = worker.Rcpt(message.Recipient)
		if err == nil {
			logger.Debug("mailer#%d-%d send command RCPT TO: %s", m.id, message.Id, message.Recipient)
			event.Client.SetTimeout(common.App.Timeout().Data)
			wc, err := worker.Data()
			if err == nil {
				logger.Debug("mailer#%d-%d send command DATA", m.id, message.Id)
				_, err = fmt.Fprint(wc, message.Body)
				if err == nil {
					wc.Close()
					logger.Debug("%s", message.Body)
					logger.Debug("mailer#%d-%d send command .", m.id, message.Id)
					// стараемся слать письма через уже созданное соединение,
					// поэтому после отправки письма не закрываем соединение
					err = worker.Reset()
					if err == nil {
						logger.Debug("mailer#%d-%d send command RSET", m.id, message.Id)
						logger.Info("mailer#%d-%d success send mail#%d", m.id, message.Id, message.Id)
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
