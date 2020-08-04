package main

import (
	"flag"
	"fmt"
	"github.com/Halfi/postmanq/application"
	"github.com/Halfi/postmanq/common"
)

func main() {
	var file, envelope, recipient string
	var numberLines int
	flag.StringVar(&file, "f", common.ExampleConfigYaml, "configuration yaml file")
	flag.StringVar(&envelope, "e", common.InvalidInputString, "necessary envelope")
	flag.StringVar(&recipient, "r", common.InvalidInputString, "necessary recipient")
	flag.Parse()

	app := application.NewGrep()
	if app.IsValidConfigFilename(file) && recipient != common.InvalidInputString {
		app.SetConfigFilename(file)
		app.RunWithArgs(envelope, recipient, numberLines)
	} else {
		fmt.Println("Usage: pmq-grep -f -r [-e]")
		flag.VisitAll(common.PrintUsage)
		fmt.Println("Example:")
		fmt.Printf("  pmq-grep -f %s -r mail@example.com\n", common.ExampleConfigYaml)
		fmt.Printf("  pmq-grep -f %s -r mail@example.com -n 1000\n", common.ExampleConfigYaml)
		fmt.Printf("  pmq-grep -f %s -r mail@example.com -e sender@mail.com\n", common.ExampleConfigYaml)
	}
}
