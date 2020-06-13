package common

type Channel struct {
	ch     chan interface{}
	closed bool
}

func NewChannel(len int) *Channel {
	return &Channel{ch: make(chan interface{}, len)}
}

func (c *Channel) Close() {
	defer func() {
		if recover() != nil {
			c.closed = true
		}
	}()

	if !c.closed {
		c.closed = true
		close(c.ch)
	}
}

func (c *Channel) Write(v interface{}) {
	defer func() {
		if recover() != nil {
			c.closed = true
		}
	}()

	if !c.closed {
		c.ch <- v
	}
}

func (c *Channel) Ch() <-chan interface{} {
	return c.ch
}
