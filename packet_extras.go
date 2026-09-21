package drissionpage

import (
	"context"
	"errors"
	"sync"

	"github.com/go-rod/rod/lib/proto"
)

var ErrNoExtraInfo = errors.New("request has no additional network information")

// PacketExtras receives the later CDP extra-info events without mutating the
// immutable DataPacket response. Some cached/failed requests have no extra info.
type PacketExtras struct {
	mu       sync.Mutex
	request  *proto.NetworkRequestWillBeSentExtraInfo
	response *proto.NetworkResponseReceivedExtraInfo
	changed  chan struct{}
	finished bool
	err      error
}

func newPacketExtras() *PacketExtras { return &PacketExtras{changed: make(chan struct{})} }
func (p *PacketExtras) signal()      { close(p.changed); p.changed = make(chan struct{}) }
func (p *PacketExtras) Request(ctx context.Context) (*proto.NetworkRequestWillBeSentExtraInfo, error) {
	for {
		p.mu.Lock()
		r, ch, err := p.request, p.changed, p.err
		p.mu.Unlock()
		if r != nil {
			return r, nil
		}
		if err != nil {
			return nil, err
		}
		select {
		case <-ch:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}
func (p *PacketExtras) Response(ctx context.Context) (*proto.NetworkResponseReceivedExtraInfo, error) {
	for {
		p.mu.Lock()
		r, ch, err := p.response, p.changed, p.err
		p.mu.Unlock()
		if r != nil {
			return r, nil
		}
		if err != nil {
			return nil, err
		}
		select {
		case <-ch:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (p *PacketExtras) end(err error) { p.mu.Lock(); defer p.mu.Unlock(); p.err = err; p.signal() }

// extraChain pairs extra events by hop order. CDP may send them before or after
// the corresponding base event; response metadata tells us which hops emit them.
type extraHop struct {
	extras          *PacketExtras
	known, expected bool
}
type extraChain struct {
	hops                        []*extraHop
	requests                    []*proto.NetworkRequestWillBeSentExtraInfo
	responses                   []*proto.NetworkResponseReceivedExtraInfo
	requestIndex, responseIndex int
	ended                       bool
}

func (c *extraChain) add() *PacketExtras {
	x := newPacketExtras()
	c.hops = append(c.hops, &extraHop{extras: x})
	return x
}
func (c *extraChain) expect(expected bool) {
	if len(c.hops) == 0 {
		return
	}
	h := c.hops[len(c.hops)-1]
	h.known = true
	h.expected = expected
	if !expected {
		h.extras.end(ErrNoExtraInfo)
	}
	c.drain()
}
func (c *extraChain) drain() {
	for c.requestIndex < len(c.hops) {
		h := c.hops[c.requestIndex]
		if !h.known {
			break
		}
		if h.expected {
			if len(c.requests) == 0 {
				break
			}
			h.extras.mu.Lock()
			h.extras.request = c.requests[0]
			h.extras.signal()
			h.extras.mu.Unlock()
			c.requests[0] = nil
			c.requests = c.requests[1:]
		}
		c.requestIndex++
	}
	for c.responseIndex < len(c.hops) {
		h := c.hops[c.responseIndex]
		if !h.known {
			break
		}
		if h.expected {
			if len(c.responses) == 0 {
				break
			}
			h.extras.mu.Lock()
			h.extras.response = c.responses[0]
			h.extras.signal()
			h.extras.mu.Unlock()
			c.responses[0] = nil
			c.responses = c.responses[1:]
		}
		c.responseIndex++
	}
}
func (c *extraChain) complete() bool {
	return c.ended && c.requestIndex == len(c.hops) && c.responseIndex == len(c.hops)
}
