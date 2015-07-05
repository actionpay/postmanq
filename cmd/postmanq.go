package main

import (
	"flag"
	"fmt"
	"github.com/AdOnWeb/postmanq/application"
	"github.com/AdOnWeb/postmanq/common"
	"github.com/davecheney/profile"
	"time"
)

func main() {
	config := profile.Config{
		CPUProfile:   true,
		MemProfile:   true,
		BlockProfile: true,
		ProfilePath:  ".",
	}
	defer profile.Start(&config).Stop()

	var file string
	flag.StringVar(&file, "f", common.ExampleConfigYaml, "configuration yaml file")
	flag.Parse()

	app := application.NewPost()
	time.AfterFunc(time.Minute * 2, func() {
		app.Events() <- common.NewApplicationEvent(common.FinishApplicationEventKind)
	})
	if app.IsValidConfigFilename(file) {
		app.SetConfigFilename(file)
		app.Run()
	} else {
		fmt.Printf("Usage: postmanq -f %s\n", common.ExampleConfigYaml)
		flag.VisitAll(common.PrintUsage)
	}
}
