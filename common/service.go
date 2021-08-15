package common

// Программа отправки почты получилась довольно сложной, т.к. она выполняет обработку и отправку писем,
// работает с диском и с сетью, ведет логирование и проверяет ограничения перед отправкой
// из - за такого насыщенного функционала, было принято решение разбить программу на логические части - сервисы
// сервис - это модуль программы, отвечающий за выполнение одной конкретной задачи, например логирование
// сервис может сам выполнять эту задачу, либо передавать выполнение задачи внутренним обработчикам

// Service сервис требующий инициализации
// данные для инициализации берутся из файла настроек
type Service interface {
	OnInit(*ApplicationEvent)
	OnFinish()
}

// EventService сервис получающий событие отправки письма
// используется сервисами для передачи события друг другу
type EventService interface {
	Event(ev *SendEvent) bool
}

// SendingService сервис принимающий участие в отправке письма
type SendingService interface {
	Service
	EventService
	OnRun()
}

// ReportService сервис принимающий участие в агрегации и выводе в консоль писем с ошибками
type ReportService interface {
	Service
	EventService
	OnShowReport()
}

// PublishService сервис перекладывающий письма из очереди в очередь
type PublishService interface {
	Service
	EventService
	OnPublish(*ApplicationEvent)
}

// GrepService сервис ищущий записи в логе по письму
type GrepService interface {
	Service
	OnGrep(*ApplicationEvent)
}
