package postmanq

type Mailer struct {

}

func NewMailer() *Mailer {
	mailer := new(Mailer)

	return mailer
}

func (this *Mailer) OnRegister(event *RegisterEvent) {
	event.Group.Done()
}

func (this *Mailer) OnInit(event *InitEvent) {

}

func (this *Mailer) OnFinish(event *FinishEvent) {

	event.Group.Done()
}

