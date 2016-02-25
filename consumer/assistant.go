package consumer

type Assistant struct {
	Binding

	Dest []string `yaml:"dest"`
}
