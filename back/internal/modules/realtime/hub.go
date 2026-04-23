package realtime

import "sync"

type Subscription struct {
	channel chan Event
	hub     *Hub
	topic   string
	once    sync.Once
}

func (subscription *Subscription) Events() <-chan Event {
	return subscription.channel
}

func (subscription *Subscription) Close() {
	subscription.once.Do(func() {
		subscription.hub.unsubscribe(subscription)
	})
}

type Hub struct {
	mu          sync.RWMutex
	subscribers map[string]map[*Subscription]struct{}
}

func NewHub() *Hub {
	return &Hub{
		subscribers: map[string]map[*Subscription]struct{}{},
	}
}

func (hub *Hub) Subscribe(topic string, bufferSize int) *Subscription {
	if bufferSize < 1 {
		bufferSize = 1
	}

	subscription := &Subscription{
		channel: make(chan Event, bufferSize),
		hub:     hub,
		topic:   topic,
	}

	hub.mu.Lock()
	defer hub.mu.Unlock()

	if _, ok := hub.subscribers[topic]; !ok {
		hub.subscribers[topic] = map[*Subscription]struct{}{}
	}

	hub.subscribers[topic][subscription] = struct{}{}
	return subscription
}

func (hub *Hub) Publish(topic string, event Event) {
	hub.mu.RLock()
	subscribers := make([]*Subscription, 0, len(hub.subscribers[topic]))
	for subscription := range hub.subscribers[topic] {
		subscribers = append(subscribers, subscription)
	}
	hub.mu.RUnlock()

	for _, subscription := range subscribers {
		select {
		case subscription.channel <- event:
		default:
			select {
			case <-subscription.channel:
			default:
			}

			select {
			case subscription.channel <- event:
			default:
			}
		}
	}
}

func (hub *Hub) unsubscribe(subscription *Subscription) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	subscribers, ok := hub.subscribers[subscription.topic]
	if !ok {
		close(subscription.channel)
		return
	}

	delete(subscribers, subscription)
	if len(subscribers) == 0 {
		delete(hub.subscribers, subscription.topic)
	}

	close(subscription.channel)
}
