// Package agent provides inter-agent communication channels.
package agent

import (
	"context"
	"sync"
	"time"
)

// Channel enables publish/subscribe communication between agents.
type Channel struct {
	mu          sync.RWMutex
	name        string
	subscribers map[string][]chan Message
	buffer      []Message
	bufferSize  int
}

// Message represents a message passed between agents.
type Message struct {
	Topic     string
	From      string
	Payload   any
	Timestamp time.Time
}

// NewChannel creates a new communication channel.
func NewChannel(name string) *Channel {
	return &Channel{
		name:        name,
		subscribers: make(map[string][]chan Message),
		buffer:      make([]Message, 0),
		bufferSize:  100,
	}
}

// Subscribe registers a listener for a topic.
func (c *Channel) Subscribe(topic string) <-chan Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	ch := make(chan Message, 10)
	c.subscribers[topic] = append(c.subscribers[topic], ch)
	return ch
}

// Unsubscribe removes a listener.
func (c *Channel) Unsubscribe(topic string, ch <-chan Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	subs := c.subscribers[topic]
	for i, sub := range subs {
		if sub == ch {
			c.subscribers[topic] = append(subs[:i], subs[i+1:]...)
			close(sub)
			break
		}
	}
}

// Publish sends a message to all subscribers of a topic.
func (c *Channel) Publish(topic string, from string, payload any) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	msg := Message{
		Topic:     topic,
		From:      from,
		Payload:   payload,
		Timestamp: time.Now(),
	}
	
	// Store in buffer
	c.buffer = append(c.buffer, msg)
	if len(c.buffer) > c.bufferSize {
		c.buffer = c.buffer[1:]
	}
	
	// Send to subscribers
	for _, ch := range c.subscribers[topic] {
		select {
		case ch <- msg:
		default:
			// Channel full, skip
		}
	}
	
	// Also send to wildcard subscribers
	for _, ch := range c.subscribers["*"] {
		select {
		case ch <- msg:
		default:
		}
	}
}

// Request sends a message and waits for a response.
func (c *Channel) Request(ctx context.Context, topic string, from string, payload any) (Message, error) {
	responseTopic := topic + ".response." + from
	respCh := c.Subscribe(responseTopic)
	defer c.Unsubscribe(responseTopic, respCh)
	
	c.Publish(topic, from, payload)
	
	select {
	case msg := <-respCh:
		return msg, nil
	case <-ctx.Done():
		return Message{}, ctx.Err()
	}
}

// History returns recent messages for a topic.
func (c *Channel) History(topic string, limit int) []Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	var result []Message
	for i := len(c.buffer) - 1; i >= 0 && len(result) < limit; i-- {
		if c.buffer[i].Topic == topic || topic == "*" {
			result = append(result, c.buffer[i])
		}
	}
	return result
}

// ChannelHub manages multiple channels.
type ChannelHub struct {
	mu       sync.RWMutex
	channels map[string]*Channel
}

// NewChannelHub creates a new channel hub.
func NewChannelHub() *ChannelHub {
	return &ChannelHub{
		channels: make(map[string]*Channel),
	}
}

// GetChannel gets or creates a channel by name.
func (h *ChannelHub) GetChannel(name string) *Channel {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if ch, ok := h.channels[name]; ok {
		return ch
	}
	
	ch := NewChannel(name)
	h.channels[name] = ch
	return ch
}

// Broadcast sends a message to all channels.
func (h *ChannelHub) Broadcast(topic string, from string, payload any) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	for _, ch := range h.channels {
		ch.Publish(topic, from, payload)
	}
}

// ChannelAgent wraps an agent with channel capabilities.
type ChannelAgent struct {
	*Agent
	id      string
	channel *Channel
}

// NewChannelAgent creates an agent connected to a channel.
func NewChannelAgent(agent *Agent, id string, channel *Channel) *ChannelAgent {
	return &ChannelAgent{
		Agent:   agent,
		id:      id,
		channel: channel,
	}
}

// Send publishes a message from this agent.
func (ca *ChannelAgent) Send(topic string, payload any) {
	ca.channel.Publish(topic, ca.id, payload)
}

// Listen subscribes to a topic.
func (ca *ChannelAgent) Listen(topic string) <-chan Message {
	return ca.channel.Subscribe(topic)
}

// Ask sends a request and waits for response.
func (ca *ChannelAgent) Ask(ctx context.Context, topic string, payload any) (Message, error) {
	return ca.channel.Request(ctx, topic, ca.id, payload)
}
