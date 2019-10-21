package notify

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	boggusEvent Event = 1
)

func TestEventnum(t *testing.T) {
	num := eventnum(boggusEvent)
	assert.Equal(t, num, uint32(1), "The value should be the same")
}

func TestNotifier(t *testing.T) {
	notifier, process := NewEventNotifier(2)

	c := make(chan *Notification, 1)
	notifier.Notify(c, boggusEvent)

	go func() {
		process(boggusEvent, 50)
	}()
	n := <-c
	assert.Equal(t, n.data, 50, "The value should be the same")
}
