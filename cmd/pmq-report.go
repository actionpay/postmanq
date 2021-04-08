package main

import (
	"flag"
	"fmt"
	"github.com/sergw3x/postmanq/application"
	"github.com/sergw3x/postmanq/common"
)

func main() {
	var file string
	flag.StringVar(&file, "f", common.ExampleConfigYaml, "configuration yaml file")
	flag.Parse()

	app := application.NewReport()
	if app.IsValidConfigFilename(file) {
		app.SetConfigFilename(file)
		app.Run()
	} else {
		fmt.Println("Usage: pmq-report -f")
		flag.VisitAll(common.PrintUsage)
		fmt.Println("Example:")
		fmt.Printf("  pmq-report -f %s\n", common.ExampleConfigYaml)
	}
}
