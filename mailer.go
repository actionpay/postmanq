package postmanq

type MailMessage struct {
	Envelope  string `json:"envelope"`
	Recipient string `json:"recipient"`
	Body      string `json:"body"`
}

type Mailer struct {
	AppsConfigs []*ConsumerApplicationConfig `yaml:"mailers"`
	messages    chan *MailMessage
}

func NewMailer() *Mailer {
	return new(Mailer)
}

func (this *Mailer) OnRegister(event *RegisterEvent) {
	this.messages = make(chan *MailMessage)
	event.MailChan = this.messages
	event.Group.Done()
}

func (this *Mailer) OnInit(event *InitEvent) {

}

func (this *Mailer) OnFinish(event *FinishEvent) {
	close(this.messages)
	event.Group.Done()
}

type MailerApplicationConfig struct {
	URI string `yaml:"uri"`
}

type MailerApplication interface {
	Run(*MailerApplicationConfig)
}

type MailerApplicationType string

const (
	MAILER_APPLICATION_TYPE_MTA MailerApplicationType = "mta"
	MAILER_APPLICATION_TYPE_SMTP                      = "smtp"
	MAILER_APPLICATION_TYPE_MX                        = "mx"
)



type SmtpMailerApplication struct {

}
