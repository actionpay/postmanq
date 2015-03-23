package postmanq

import (
	yaml "gopkg.in/yaml.v2"
	"fmt"
	"os"
	"bufio"
	"regexp"
	"strings"
	"bytes"
)

var (
	searcher *Searcher
	mailIdRegex        = regexp.MustCompile(`mail#(\d)+`)
	limiterIdRegex     = regexp.MustCompile(`limiter#(\d)+`)
	connectorIdRegex   = regexp.MustCompile(`connector#(\d)+`)
	mailerIdRegex      = regexp.MustCompile(`mailer#(\d)+`)
)

type Searcher struct {
	Output  string   `yaml:"logOutput"` // название вывода логов
	logFile *os.File
}

func SearcherOnce() *Searcher {
	if searcher == nil {
		searcher = new(Searcher)
	}
	return searcher
}

func (this *Searcher) OnInit(event *ApplicationEvent) {
	var err error
	err = yaml.Unmarshal(event.Data, this)
	if err == nil {
		if filenameRegex.MatchString(this.Output) {
			this.logFile, err = os.OpenFile(this.Output, os.O_RDONLY, os.ModePerm)
			if err != nil {
				fmt.Println(err)
				app.Events() <- NewApplicationEvent(APPLICATION_EVENT_KIND_FINISH)
			}
		} else {
			fmt.Println("logOutput should be a file")
			app.Events() <- NewApplicationEvent(APPLICATION_EVENT_KIND_FINISH)
		}
	} else {
		fmt.Println("searcher can't unmarshal config file")
		app.Events() <- NewApplicationEvent(APPLICATION_EVENT_KIND_FINISH)
	}
}

func (this *Searcher) OnGrep(event *ApplicationEvent) {
	scanner := bufio.NewScanner(this.logFile)
	scanner.Split(bufio.ScanLines)
	lines := make([]string, 0)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	linesLen := len(lines)
	if event.GetIntArg("numberLines") > INVALID_INPUT_INT && event.GetIntArg("numberLines") < linesLen {
		lines = lines[linesLen - event.GetIntArg("numberLines"):]
	}

	var expr string
	hasEnvelope := len(event.GetStringArg("envelope")) > 0
	if hasEnvelope {
		expr = fmt.Sprintf("envelope - %s, recipient - %s to mailer", event.GetStringArg("envelope"), event.GetStringArg("recipient"))
	} else {
		expr = fmt.Sprintf("recipient - %s to mailer", event.GetStringArg("recipient"))
	}

	mailRegex, err := regexp.Compile(expr)
	if err == nil {
		for i, line := range lines {
			if mailRegex.MatchString(line) {
				go this.grep(mailIdRegex.FindString(line), lines[i:])
			}
		}
	} else {
		if hasEnvelope {
			fmt.Println("invalid recipient")
		} else {
			fmt.Println("invalid recipient or envelope")
		}
	}

	app.Events() <- NewApplicationEvent(APPLICATION_EVENT_KIND_FINISH)
}

func (this *Searcher) grep(mailId string, lines []string) {
	var limiterId, connectorId, mailerId  string
	mailRegex, _ := regexp.Compile(mailId)
	out := new(bytes.Buffer)
	out.WriteString("\n")

	for _, line := range lines {
		if mailRegex.MatchString(line) {
			out.WriteString(line)
			out.WriteString("\n")

			if limiterIdRegex.MatchString(line) {
				limiterId = limiterIdRegex.FindString(line)
			}
			if connectorIdRegex.MatchString(line) {
				limiterId = INVALID_INPUT_STRING
				connectorId = connectorIdRegex.FindString(line)
			}
			if mailerIdRegex.MatchString(line) {
				connectorId = INVALID_INPUT_STRING
				mailerId = mailerIdRegex.FindString(line)
			}

			if strings.Contains(line, "sending error") || strings.Contains(line, "success send") {
				mailerId = INVALID_INPUT_STRING
			}
		} else {
			if len(limiterId) > 0 && strings.Contains(line, limiterId) {
				out.WriteString(line)
				out.WriteString("\n")
			}
			if len(connectorId) > 0 && strings.Contains(line, connectorId) {
				out.WriteString(line)
				out.WriteString("\n")
			}
			if len(mailerId) > 0 && strings.Contains(line, mailerId) {
				out.WriteString(line)
				out.WriteString("\n")
			}
		}
	}

	fmt.Print(out.String())
}

func (this *Searcher) OnFinish(event *ApplicationEvent) {
	this.logFile.Close()
}
