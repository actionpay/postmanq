package main

import (
	"flag"
	"github.com/AdOnWeb/postmanq"
	"fmt"
)

func main() {
	var file, srcQueue, destQueue, host, envelope, recipient string
	var code int
	flag.StringVar(&file, "f", postmanq.EXAMPLE_CONFIG_YAML, "configuration yaml file")
	flag.StringVar(&srcQueue, "s", postmanq.INVALID_INPUT_STRING, "source queue")
	flag.StringVar(&destQueue, "d", postmanq.INVALID_INPUT_STRING, "destination queue")
	flag.StringVar(&host, "h", postmanq.INVALID_INPUT_STRING, "amqp server hostname")
	flag.IntVar(&code, "c", postmanq.INVALID_INPUT_INT, "error code")
	flag.StringVar(&envelope, "e", postmanq.INVALID_INPUT_STRING, "necessary envelope")
	flag.StringVar(&recipient, "r", postmanq.INVALID_INPUT_STRING, "necessary recipient")
	flag.Parse()

	app := postmanq.NewPublishApplication()
	if app.IsValidConfigFilename(file) &&
		srcQueue != postmanq.INVALID_INPUT_STRING &&
		destQueue != postmanq.INVALID_INPUT_STRING {
		app.SetConfigFilename(file)
		app.RunWithArgs(srcQueue, destQueue, host, code, envelope, recipient)
	} else {
		fmt.Println("Usage: pmq-publish -f -s -d [-h] [-c] [-e] [-r]")
		flag.VisitAll(postmanq.PrintUsage)
		fmt.Println("Example:")
		fmt.Printf("  pmq-publish -f %s -s outbox.fail -d outbox\n", postmanq.EXAMPLE_CONFIG_YAML)
		fmt.Printf("  pmq-publish -f %s -s outbox.fail -d outbox -h example.com\n", postmanq.EXAMPLE_CONFIG_YAML)
		fmt.Printf("  pmq-publish -f %s -s outbox.fail -d outbox -c 554\n", postmanq.EXAMPLE_CONFIG_YAML)
	}
}
