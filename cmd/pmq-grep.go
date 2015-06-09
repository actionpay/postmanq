package main

import (
	"flag"
	"fmt"
	"github.com/AdOnWeb/postmanq"
)

func main() {
	var file, envelope, recipient string
	var numberLines int
	flag.StringVar(&file, "f", postmanq.ExampleConfigYaml, "configuration yaml file")
	flag.StringVar(&envelope, "e", postmanq.InvalidInputString, "necessary envelope")
	flag.StringVar(&recipient, "r", postmanq.InvalidInputString, "necessary recipient")
	flag.IntVar(&numberLines, "n", postmanq.InvalidInputInt, "output the last n lines")
	flag.Parse()

	app := postmanq.NewGrepApplication()
	if app.IsValidConfigFilename(file) && recipient != postmanq.InvalidInputString {
		app.SetConfigFilename(file)
		app.RunWithArgs(envelope, recipient, numberLines)
	} else {
		fmt.Println("Usage: pmq-grep -f -r [-e] [-n]")
		flag.VisitAll(postmanq.PrintUsage)
		fmt.Println("Example:")
		fmt.Printf("  pmq-grep -f %s -r mail@example.com\n", postmanq.ExampleConfigYaml)
		fmt.Printf("  pmq-grep -f %s -r mail@example.com -n 1000\n", postmanq.ExampleConfigYaml)
		fmt.Printf("  pmq-grep -f %s -r mail@example.com -e sender@mail.com\n", postmanq.ExampleConfigYaml)
	}
}
