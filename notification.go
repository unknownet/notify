package notify

import (
	"sync"
)

type Notification struct {
	event Event
	data  interface{}
}

func (n *Notification) Event() Event {
	return n.event
}

func (n *Notification) Data() interface{} {
	return n.data
}

type Event uint32

type handler struct {
	mask []uint32
}

func (h *handler) want(event uint32) bool {
	return (h.mask[event/32]>>uint(event&31))&1 != 0
}

func (h *handler) set(event uint32) {
	h.mask[event/32] |= 1 << uint(event&31)
}

func (h *handler) clear(event uint32) {
	h.mask[event/32] &^= 1 << uint(event&31)
}

type NotifierConfig struct {
	enableEvent  func(uint32)
	disableEvent func(uint32)
}

type NotifierOption func(*NotifierConfig)

func EnableEvent(fn func(uint32)) NotifierOption {
	return func(n *NotifierConfig) {
		n.enableEvent = fn
	}
}

func DisableEvent(fn func(uint32)) NotifierOption {
	return func(n *NotifierConfig) {
		n.disableEvent = fn
	}
}

type Notifier interface {
	Notify(c chan<- *Notification, events ...Event)
	Reset(events ...Event)
	Stop(c chan<- *Notification)
}

type EventNotifier struct {
	sync.Mutex
	num      uint32
	c        *NotifierConfig
	m        map[chan<- *Notification]*handler
	ref      []int64
	stopping []stopping
}

func defaultEventNotifierConfig() *NotifierConfig {
	return &NotifierConfig{
		enableEvent:  nil,
		disableEvent: nil,
	}
}

func NewEventNotifier(num uint32, opts ...NotifierOption) (*EventNotifier, func(event Event, data interface{})) {

	config := defaultEventNotifierConfig()
	for _, opt := range opts {
		opt(config)
	}

	notifier := &EventNotifier{
		num: num,
		m:   make(map[chan<- *Notification]*handler),
		ref: make([]int64, num),
		c:   config,
	}
	return notifier, notifier.process
}

type stopping struct {
	c chan<- *Notification
	h *handler
}

func eventnum(event Event) uint32 {
	return uint32(event)
}

// func (d *Notifier) cancel(notifs []Notification, action func(int)) {
// 	n.Lock()
// 	defer n.Unlock()

// 	remove := func(n int) {
// 		var zerohandler handler

// 		for c, h := range n.m {
// 			if h.want(n) {
// 				n.ref[n]--
// 				h.clear(n)
// 				if h.mask == zerohandler.mask {
// 					delete(n.m, c)
// 				}
// 			}
// 		}

// 		action(n)
// 	}

// 	if len(notifs) == 0 {
// 		for n := 0; n < n.num; n++ {
// 			remove(n)
// 		}
// 	} else {
// 		for _, s := range notifs {
// 			remove(notifnum(s))
// 		}
// 	}
// }

// func (d *Notifier) Ignore(notif ...Notification) {
// 	n.cancel(notif, ignoreNotification)
// }

// func (d *Notifier) Ignored(notif Notification) bool {
// 	num := notifnum(notif)
// 	return num >= 0 && notificationIgnored(num)
// }

func (e *EventNotifier) Notify(c chan<- *Notification, events ...Event) {
	if c == nil {
		panic("notifier: Notify using nil channel")
	}

	e.Lock()
	defer e.Unlock()

	h := e.m[c]
	if h == nil {
		if e.m == nil {
			e.m = make(map[chan<- *Notification]*handler)
		}
		h = &handler{
			mask: make([]uint32, (e.num+31)/32),
		}
		e.m[c] = h
	}

	add := func(num uint32) {
		if num < 0 {
			return
		}
		if !h.want(num) {
			h.set(num)
			if e.ref[num] == 0 && e.c.enableEvent != nil {
				e.c.enableEvent(num)
			}
			e.ref[num]++
		}
	}

	if len(events) == 0 {
		for num := uint32(0); num < e.num; num++ {
			add(num)
		}
	} else {
		for _, e := range events {
			add(eventnum(e))
		}
	}
}

// func (d *Notifier) Reset(notif ...Notification) {
// 	n.cancel(notif, disableNotification)
// }

func (e *EventNotifier) Reset(events ...Event) {
	e.Lock()
	defer e.Unlock()

	remove := func(num uint32) {
		for c, h := range e.m {
			if h.want(num) {
				e.ref[num]--
				h.clear(num)

				hasZeroHandler := true
				for _, sub := range h.mask {
					if sub != 0 {
						hasZeroHandler = false
						break
					}
				}
				if hasZeroHandler {
					delete(e.m, c)
				}
			}
		}
		// action(n)
	}

	if len(events) == 0 {
		for num := uint32(0); num < e.num; num++ {
			remove(num)
		}
	} else {
		for _, ev := range events {
			remove(eventnum(ev))
		}
	}
}

func (e *EventNotifier) Stop(c chan<- *Notification) {
	e.Lock()

	h := e.m[c]
	if h == nil {
		e.Unlock()
		return
	}
	delete(e.m, c)

	for num := uint32(0); num < e.num; num++ {
		if h.want(num) {
			e.ref[num]--
			if e.ref[num] == 0 && e.c.disableEvent != nil {
				e.c.disableEvent(num)
			}
		}
	}

	e.stopping = append(e.stopping, stopping{c, h})

	e.Unlock()

	e.Lock()

	for num, s := range e.stopping {
		if s.c == c {
			e.stopping = append(e.stopping[:num], e.stopping[num+1:]...)
			break
		}
	}

	e.Unlock()
}

func (e *EventNotifier) process(event Event, data interface{}) {
	num := eventnum(event)
	if num < 0 {
		return
	}

	e.Lock()
	defer e.Unlock()

	notification := &Notification{
		event: event,
		data:  data,
	}

	for c, h := range e.m {
		if h.want(num) {
			// send but do not block for it
			select {
			case c <- notification:
			default:
			}
		}
	}

	// Avoid the race mentioned in Stop.
	for _, s := range e.stopping {
		if s.h.want(num) {
			select {
			case s.c <- notification:
			default:
			}
		}
	}
}
