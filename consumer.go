package postmanq

type Consumer struct {
	URI string          `yaml:"uri"`
	Bindings []*Binding `yaml:"bindings"`
}

type Binding struct {
	Name     string       `yaml:"name"`
	Exchange string       `yaml:"exchange"`
	Queue    string       `yaml:"queue"`
	Type     ExchangeType `yaml:"type"`
}

type ExchangeType string

const (
	MAILBOX_DIRECT ExchangeType = "direct"
	MAILBOX_FANOUT              = "fanout"
	MAILBOX_TOPIC               = "topic"
)

