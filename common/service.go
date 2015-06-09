package common

// Программа отправки почты получилась довольно сложной, т.к. она выполняет обработку и отправку писем,
// работает с диском и с сетью, ведет логирование и проверяет ограничения перед отправкой.
// Из - за такого насыщенного функционала, было принято решение разбить программу на логические части - сервисы.
// Сервис - это модуль программы, отвечающий за выполнение одной конкретной задачи, например логирование.
// Сервис может сам выполнять эту задачу, либо управлять выполнением задачи.
type Service interface {
	OnInit(*ApplicationEvent)
}

type SendingService interface {
	Service
	OnRun()
	OnFinish()
	Events() chan *SendEvent
}

type ReportService interface {
	Service
	OnShowReport()
}

type PublishService interface {
	Service
	OnPublish(*ApplicationEvent)
}

type GrepService interface {
	Service
	OnGrep(*ApplicationEvent)
}
