package notify

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
