package main

import (
	"encoding/json"
	"github.com/streadway/amqp"
	"log"
)

type MailMessage struct {
	Envelope     string `json:"envelope"`               // отправитель
	Recipient    string `json:"recipient"`              // получатель
	Body         string `json:"body"`                   // тело письма
	EnvelopeName string `json:"envelopeName,omitempty"` // отправитель / имя
	ContentType  string `json:"contentType,omitempty"`  // тип контента
	Subject      string `json:"subject,omitempty"`      // тема письма
	TemplateName string `json:"tplName,omitempty"`      //шаблон который использовать
}

func main() {
	// Here we connect to RabbitMQ or send a message if there are any errors connecting.
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Fatalf("%s: %s", "Failed to connect to RabbitMQ", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("%s: %s", "Failed to open a channel", err)
	}
	defer ch.Close()

	// We create a Queue to send the message to.
	q, err := ch.QueueDeclare(
		"postmanq", // name
		true,       // durable
		false,      // delete when unused
		false,      // exclusive
		false,      // no-wait
		nil,        // arguments
	)
	if err != nil {
		log.Fatalf("%s: %s", "Failed to declare a queue", err)
	}

	msg := MailMessage{
		"info@tevex.ru",
		"serg@tevex.ru",
		"<h1>письмо с заголовками и содержимым<h1><h2>письмо с заголовками и содержимым<h2>",
		"отправун",
		"text/html",
		"тест",
		"template",
	}
	body, err := json.Marshal(msg)
	if err != nil {
		log.Fatal(err)
	}

	//body := "{\"envelope\": \"info@tevex.ru\", \"recipient\": \"serg@tevex.ru\", \"body\": \"письмо с заголовками и содержимым\"}"
	err = ch.Publish(
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        body,
		})
	// If there is an error publishing the message, a log will be displayed in the terminal.
	if err != nil {
		log.Fatalf("%s: %s", "Failed to publish a message", err)
	}
	log.Printf(" [x] Congrats, sending message: %s", body)
}
