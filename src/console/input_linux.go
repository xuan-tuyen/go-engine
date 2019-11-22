package console

type ConsoleInput struct {
}

func (ci *ConsoleInput) Init() error {
	return nil
}

func (ci *ConsoleInput) Close() {
}

func (ci *ConsoleInput) PollEvent() *EventKey {
	return nil
}
