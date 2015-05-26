package main

import (
	"flag"
	"github.com/AdOnWeb/postmanq"
	"fmt"
)

func main() {
	var file, srcQueue, destQueue, host, envelope, recipient string
	var code int
	flag.StringVar(&file, "f", postmanq.ExampleConfigYaml, "configuration yaml file")
	flag.StringVar(&srcQueue, "s", postmanq.InvalidInputString, "source queue")
	flag.StringVar(&destQueue, "d", postmanq.InvalidInputString, "destination queue")
	flag.StringVar(&host, "h", postmanq.InvalidInputString, "amqp server hostname")
	flag.IntVar(&code, "c", postmanq.InvalidInputInt, "error code")
	flag.StringVar(&envelope, "e", postmanq.InvalidInputString, "necessary envelope")
	flag.StringVar(&recipient, "r", postmanq.InvalidInputString, "necessary recipient")
	flag.Parse()

	app := postmanq.NewPublishApplication()
	if app.IsValidConfigFilename(file) &&
		srcQueue != postmanq.InvalidInputString &&
		destQueue != postmanq.InvalidInputString {
		app.SetConfigFilename(file)
		app.RunWithArgs(srcQueue, destQueue, host, code, envelope, recipient)
	} else {
		fmt.Println("Usage: pmq-publish -f -s -d [-h] [-c] [-e] [-r]")
		flag.VisitAll(postmanq.PrintUsage)
		fmt.Println("Example:")
		fmt.Printf("  pmq-publish -f %s -s outbox.fail -d outbox\n", postmanq.ExampleConfigYaml)
		fmt.Printf("  pmq-publish -f %s -s outbox.fail -d outbox -h example.com\n", postmanq.ExampleConfigYaml)
		fmt.Printf("  pmq-publish -f %s -s outbox.fail -d outbox -c 554\n", postmanq.ExampleConfigYaml)
	}
}
