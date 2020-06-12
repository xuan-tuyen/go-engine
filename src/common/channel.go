package common

import "time"

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
		close(c.ch)
	}
}

func (c *Channel) Write(v interface{}) {
	defer func() {
		if recover() != nil {
			c.closed = true
		}
	}()

	for !c.closed {
		select {
		case c.ch <- v:
			break
		case <-time.After(time.Second):
		}
	}
}

func (c *Channel) Ch() <-chan interface{} {
	return c.ch
}
