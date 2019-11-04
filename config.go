package notify

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

func defaultNotifierConfig() *NotifierConfig {
	return &NotifierConfig{
		enableEvent:  nil,
		disableEvent: nil,
	}
}
