package main

import (
	"github.com/byorty/postmanq"
	"flag"
)


func main() {
	var file string
	flag.StringVar(&file, "f", postmanq.EXAMPLE_CONFIG_YAML, "configuration yaml file")
	flag.Parse()

	app := postmanq.NewApplication()
	app.ConfigFilename = file
	app.Run()
}
