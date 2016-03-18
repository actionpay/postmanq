package main

import (
	"flag"
	"fmt"
	"github.com/actionpay/postmanq/application"
	"github.com/actionpay/postmanq/common"
)

func main() {
	var file string
	flag.StringVar(&file, "f", common.ExampleConfigYaml, "configuration yaml file")
	flag.Parse()

	app := application.NewPost()
	if app.IsValidConfigFilename(file) {
		app.SetConfigFilename(file)
		app.Run()
	} else {
		fmt.Printf("Usage: postmanq -f %s\n", common.ExampleConfigYaml)
		flag.VisitAll(common.PrintUsage)
	}
}
