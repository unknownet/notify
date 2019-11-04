package notify

import "sync"

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

type Notifiable interface {
	Notify(c chan<- *Notification, events ...Event)
	Reset(events ...Event)
	Stop(c chan<- *Notification)
}

type Notifier struct {
	sync.Mutex
	num      uint32
	c        *NotifierConfig
	m        map[chan<- *Notification]*handler
	ref      []int64
	stopping []stopping
}

type Process func(event Event, data interface{})

func NewNotifier(num uint32, opts ...NotifierOption) (*Notifier, Process) {

	config := defaultNotifierConfig()
	for _, opt := range opts {
		opt(config)
	}

	notifier := &Notifier{
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

func (n *Notifier) Notify(c chan<- *Notification, events ...Event) {
	if c == nil {
		panic("notifier: Notify using nil channel")
	}

	n.Lock()
	defer n.Unlock()

	h := n.m[c]
	if h == nil {
		if n.m == nil {
			n.m = make(map[chan<- *Notification]*handler)
		}
		h = &handler{
			mask: make([]uint32, (n.num+31)/32),
		}
		n.m[c] = h
	}

	add := func(num uint32) {
		if num < 0 {
			return
		}
		if !h.want(num) {
			h.set(num)
			if n.ref[num] == 0 && n.c.enableEvent != nil {
				n.c.enableEvent(num)
			}
			n.ref[num]++
		}
	}

	if len(events) == 0 {
		for num := uint32(0); num < n.num; num++ {
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

func (n *Notifier) Reset(events ...Event) {
	n.Lock()
	defer n.Unlock()

	remove := func(num uint32) {
		for c, h := range n.m {
			if h.want(num) {
				n.ref[num]--
				h.clear(num)

				hasZeroHandler := true
				for _, sub := range h.mask {
					if sub != 0 {
						hasZeroHandler = false
						break
					}
				}
				if hasZeroHandler {
					delete(n.m, c)
				}
			}
		}
		// action(n)
	}

	if len(events) == 0 {
		for num := uint32(0); num < n.num; num++ {
			remove(num)
		}
	} else {
		for _, e := range events {
			remove(eventnum(e))
		}
	}
}

func (n *Notifier) Stop(c chan<- *Notification) {
	n.Lock()

	h := n.m[c]
	if h == nil {
		n.Unlock()
		return
	}
	delete(n.m, c)

	for num := uint32(0); num < n.num; num++ {
		if h.want(num) {
			n.ref[num]--
			if n.ref[num] == 0 && n.c.disableEvent != nil {
				n.c.disableEvent(num)
			}
		}
	}

	n.stopping = append(n.stopping, stopping{c, h})

	n.Unlock()

	n.Lock()

	for num, s := range n.stopping {
		if s.c == c {
			n.stopping = append(n.stopping[:num], n.stopping[num+1:]...)
			break
		}
	}

	n.Unlock()
}

func (n *Notifier) process(event Event, data interface{}) {
	num := eventnum(event)
	if num < 0 {
		return
	}

	n.Lock()
	defer n.Unlock()

	notification := &Notification{
		event: event,
		data:  data,
	}

	for c, h := range n.m {
		if h.want(num) {
			// send but do not block for it
			select {
			case c <- notification:
			default:
			}
		}
	}

	// Avoid the race mentioned in Stop.
	for _, s := range n.stopping {
		if s.h.want(num) {
			select {
			case s.c <- notification:
			default:
			}
		}
	}
}
