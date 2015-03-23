package main

import (
	"github.com/AdOnWeb/postmanq"
	"flag"
	"fmt"
)

func main() {
	var file, envelope, recipient string
	var numberLines int
	flag.StringVar(&file, "f", postmanq.EXAMPLE_CONFIG_YAML, "configuration yaml file")
	flag.StringVar(&envelope, "e", postmanq.INVALID_INPUT_STRING, "necessary envelope")
	flag.StringVar(&recipient, "r", postmanq.INVALID_INPUT_STRING, "necessary recipient")
	flag.IntVar(&numberLines, "n", postmanq.INVALID_INPUT_INT, "output the last n lines")
	flag.Parse()

	app := postmanq.NewGrepApplication()
	if app.IsValidConfigFilename(file) && recipient != postmanq.INVALID_INPUT_STRING {
		app.SetConfigFilename(file)
		app.RunWithArgs(envelope, recipient, numberLines)
	} else {
		fmt.Println("Usage: pmq-grep -f -r [-e] [-n]")
		flag.VisitAll(postmanq.PrintUsage)
		fmt.Println("Example:")
		fmt.Printf("  pmq-grep -f %s -r mail@example.com\n", postmanq.EXAMPLE_CONFIG_YAML)
		fmt.Printf("  pmq-grep -f %s -r mail@example.com -n 1000\n", postmanq.EXAMPLE_CONFIG_YAML)
		fmt.Printf("  pmq-grep -f %s -r mail@example.com -e sender@mail.com\n", postmanq.EXAMPLE_CONFIG_YAML)
	}
}

