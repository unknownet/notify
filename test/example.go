package main

import (
	"fmt"
	"sync"

	"github.com/unknownet/notify"
)

const (
	abortedEvent notify.Event = iota
	finishEvent
)

type NotifiableStruct struct {
	*notify.Notifier
	process notify.Process
}

func newNotifiableStruct() *NotifiableStruct {

	notifier, process := notify.NewNotifier(2)

	return &NotifiableStruct{
		Notifier: notifier,
		process:  process,
	}
}

func main() {
	c := make(chan *notify.Notification, 1)
	notifiable := newNotifiableStruct()
	notifiable.Notify(c, finishEvent)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		notif := <-c
		fmt.Printf("Notification value %d\n", notif.Data())
		wg.Done()
	}()

	notifiable.process(abortedEvent, 2)
	notifiable.process(finishEvent, 1)
	wg.Wait()
}
