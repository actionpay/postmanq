package main

import (
	"flag"
	"github.com/AdOnWeb/postmanq"
	"fmt"
)

func main() {
	var file string
	flag.StringVar(&file, "f", postmanq.EXAMPLE_CONFIG_YAML, "configuration yaml file")
	flag.Parse()

	app := postmanq.NewReportApplication()
	if app.IsValidConfigFilename(file) {
		app.SetConfigFilename(file)
		app.Run()
	} else {
		fmt.Println("Usage: pmq-report -f")
		flag.VisitAll(postmanq.PrintUsage)
		fmt.Println("Example:")
		fmt.Printf("  pmq-report -f %s\n", postmanq.EXAMPLE_CONFIG_YAML)
	}
}

