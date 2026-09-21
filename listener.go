package drissionpage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

type ListenFilter struct {
	URLs          []string
	Regex         bool
	Methods       []string
	ResourceTypes []string
	// BufferSize bounds each output queue; zero uses 4096. Oldest events are dropped on overflow.
	BufferSize int
}
type DataPacket struct {
	TabID        string
	RequestID    string
	URL          string
	Method       string
	ResourceType string
	Request      *proto.NetworkRequest
	Response     *proto.NetworkResponse
	Body         []byte
	Failure      string
	BodyError    error
	Timestamp    time.Time
	Extras       *PacketExtras
}

func (p *DataPacket) JSON(out any) error { return json.Unmarshal(p.Body, out) }

type StreamMessage struct {
	TabID                                          string
	RequestID, URL, Kind, EventName, EventID, Data string
	Opcode                                         float64
	Timestamp                                      time.Time
}
type Listener struct {
	packets *eventQueue[*DataPacket]
	streams *eventQueue[*StreamMessage]
	cancel  context.CancelFunc
	paused  atomic.Bool
	active  atomic.Int64
}

func (t *ChromiumTab) Listen(ctx context.Context, filter ListenFilter) (*Listener, error) {
	var patterns []*regexp.Regexp
	for _, s := range filter.URLs {
		if filter.Regex {
			r, e := regexp.Compile(s)
			if e != nil {
				return nil, e
			}
			patterns = append(patterns, r)
		}
	}
	matches := func(target, method, kind string) bool {
		if len(filter.Methods) > 0 {
			ok := false
			for _, m := range filter.Methods {
				if strings.EqualFold(m, method) {
					ok = true
				}
			}
			if !ok {
				return false
			}
		}
		if len(filter.ResourceTypes) > 0 {
			ok := false
			for _, r := range filter.ResourceTypes {
				if strings.EqualFold(r, kind) {
					ok = true
				}
			}
			if !ok {
				return false
			}
		}
		if len(filter.URLs) == 0 {
			return true
		}
		for i, s := range filter.URLs {
			if filter.Regex {
				if patterns[i].MatchString(target) {
					return true
				}
			} else if strings.Contains(target, s) {
				return true
			}
		}
		return false
	}
	life, cancel := context.WithCancel(ctx)
	p := t.page.Context(life)
	l := &Listener{packets: newEventQueue[*DataPacket](), streams: newEventQueue[*StreamMessage](), cancel: cancel}
	l.packets.setLimit(filter.BufferSize)
	l.streams.setLimit(filter.BufferSize)
	pending := map[proto.NetworkRequestID]*DataPacket{}
	extras := map[proto.NetworkRequestID]*extraChain{}
	getChain := func(id proto.NetworkRequestID) *extraChain {
		x := extras[id]
		if x == nil {
			x = &extraChain{}
			extras[id] = x
		}
		return x
	}
	clean := func(id proto.NetworkRequestID) {
		if x := extras[id]; x != nil && x.complete() {
			delete(extras, id)
		}
	}
	sockets := map[proto.NetworkRequestID]string{}
	if e := (proto.NetworkEnable{}).Call(p); e != nil {
		cancel()
		return nil, e
	}
	finish := func(id proto.NetworkRequestID, failure string) {
		if x := extras[id]; x != nil {
			x.ended = true
			if len(x.hops) > 0 && !x.hops[len(x.hops)-1].known {
				x.expect(false)
			}
			clean(id)
		}
		packet, ok := pending[id]
		if !ok {
			return
		}
		delete(pending, id)
		l.active.Add(-1)
		packet.Failure = failure
		if failure == "" {
			body, e := (proto.NetworkGetResponseBody{RequestID: id}).Call(p)
			if e != nil {
				packet.BodyError = e
			} else if body.Base64Encoded {
				packet.Body, packet.BodyError = base64.StdEncoding.DecodeString(body.Body)
			} else {
				packet.Body = []byte(body.Body)
			}
		}
		if !l.paused.Load() {
			l.packets.push(packet)
		}
	}
	wait := p.EachEvent(
		func(e *proto.NetworkRequestWillBeSent) {
			chain := getChain(e.RequestID)
			if e.RedirectResponse != nil {
				chain.expect(e.RedirectHasExtraInfo)
			}
			packetExtras := chain.add()
			if old, ok := pending[e.RequestID]; ok {
				delete(pending, e.RequestID)
				l.active.Add(-1)
				old.Response = e.RedirectResponse
				if !l.paused.Load() {
					l.packets.push(old)
				}
			}
			if l.paused.Load() || !matches(e.Request.URL, e.Request.Method, string(e.Type)) {
				return
			}
			pending[e.RequestID] = &DataPacket{RequestID: string(e.RequestID), URL: e.Request.URL, Method: e.Request.Method, ResourceType: string(e.Type), Request: e.Request, Timestamp: time.Now(), Extras: packetExtras}
			l.active.Add(1)
		},
		func(e *proto.NetworkResponseReceived) {
			getChain(e.RequestID).expect(e.HasExtraInfo)
			if packet := pending[e.RequestID]; packet != nil {
				packet.Response = e.Response
			}
		},
		func(e *proto.NetworkRequestWillBeSentExtraInfo) {
			x := getChain(e.RequestID)
			x.requests = append(x.requests, e)
			x.drain()
			clean(e.RequestID)
		},
		func(e *proto.NetworkResponseReceivedExtraInfo) {
			x := getChain(e.RequestID)
			x.responses = append(x.responses, e)
			x.drain()
			clean(e.RequestID)
		},
		func(e *proto.NetworkLoadingFinished) { finish(e.RequestID, "") },
		func(e *proto.NetworkLoadingFailed) { finish(e.RequestID, e.ErrorText) },
		func(e *proto.NetworkWebSocketCreated) { sockets[e.RequestID] = e.URL },
		func(e *proto.NetworkWebSocketFrameReceived) {
			u := sockets[e.RequestID]
			if !l.paused.Load() && matches(u, "GET", "WebSocket") {
				l.streams.push(&StreamMessage{RequestID: string(e.RequestID), URL: u, Kind: "websocket-received", Data: e.Response.PayloadData, Opcode: e.Response.Opcode, Timestamp: time.Now()})
			}
		},
		func(e *proto.NetworkWebSocketFrameSent) {
			u := sockets[e.RequestID]
			if !l.paused.Load() && matches(u, "GET", "WebSocket") {
				l.streams.push(&StreamMessage{RequestID: string(e.RequestID), URL: u, Kind: "websocket-sent", Data: e.Response.PayloadData, Opcode: e.Response.Opcode, Timestamp: time.Now()})
			}
		},
		func(e *proto.NetworkWebSocketClosed) { delete(sockets, e.RequestID) },
		func(e *proto.NetworkEventSourceMessageReceived) {
			if packet := pending[e.RequestID]; packet != nil && !l.paused.Load() {
				l.streams.push(&StreamMessage{RequestID: string(e.RequestID), URL: packet.URL, Kind: "sse", EventName: e.EventName, EventID: e.EventID, Data: e.Data, Timestamp: time.Now()})
			}
		},
	)
	go func() {
		defer l.packets.close()
		defer l.streams.close()
		wait()
		for _, chain := range extras {
			for _, hop := range chain.hops {
				hop.extras.end(ErrClosed)
			}
		}
	}()
	return l, nil
}
func (l *Listener) Wait(ctx context.Context, count int) ([]*DataPacket, error) {
	out := make([]*DataPacket, 0)
	for len(out) < count {
		p, e := l.packets.pop(ctx)
		if e != nil {
			return out, e
		}
		out = append(out, p)
	}
	return out, nil
}
func (l *Listener) Next(ctx context.Context) (*DataPacket, error)          { return l.packets.pop(ctx) }
func (l *Listener) NextStream(ctx context.Context) (*StreamMessage, error) { return l.streams.pop(ctx) }
func (l *Listener) Pause(clear bool) {
	l.paused.Store(true)
	if clear {
		l.Clear()
	}
}
func (l *Listener) Resume() { l.paused.Store(false) }
func (l *Listener) Clear()  { l.packets.clear(); l.streams.clear() }
func (l *Listener) Stop()   { l.cancel(); l.packets.close(); l.streams.close() }
func (l *Listener) WaitSilent(ctx context.Context, quiet time.Duration) error {
	since := time.Now()
	for {
		if l.active.Load() > 0 {
			since = time.Now()
		}
		if time.Since(since) >= quiet {
			return nil
		}
		if e := waitDuration(ctx, 20*time.Millisecond); e != nil {
			return e
		}
	}
}

type ConsoleMessage struct {
	Type      string
	Args      []json.RawMessage
	Timestamp time.Time
	Stack     *proto.RuntimeStackTrace
}
type Console struct {
	queue  *eventQueue[*ConsoleMessage]
	cancel context.CancelFunc
}

func (t *ChromiumTab) Console(ctx context.Context) (*Console, error) {
	life, cancel := context.WithCancel(ctx)
	p := t.page.Context(life)
	if e := (proto.RuntimeEnable{}).Call(p); e != nil {
		cancel()
		return nil, e
	}
	c := &Console{newEventQueue[*ConsoleMessage](), cancel}
	wait := p.EachEvent(func(e *proto.RuntimeConsoleAPICalled) {
		m := &ConsoleMessage{Type: string(e.Type), Timestamp: time.Now(), Stack: e.StackTrace}
		for _, arg := range e.Args {
			v, err := arg.Value.MarshalJSON()
			if arg.ObjectID != "" {
				result, callErr := (proto.RuntimeCallFunctionOn{ObjectID: arg.ObjectID, ReturnByValue: true, Silent: true, FunctionDeclaration: consoleSnapshotJS}).Call(p)
				if callErr == nil && result.ExceptionDetails == nil {
					v, err = result.Result.Value.MarshalJSON()
				} else {
					v, err = json.Marshal(arg.Description)
				}
			} else if arg.UnserializableValue != "" {
				v, err = json.Marshal(arg.UnserializableValue)
			}
			if err != nil {
				v, _ = json.Marshal(arg.Description)
			}
			m.Args = append(m.Args, v)
		}
		c.queue.push(m)
	})
	go func() { defer c.queue.close(); wait() }()
	return c, nil
}
func (c *Console) Next(ctx context.Context) (*ConsoleMessage, error) { return c.queue.pop(ctx) }
func (c *Console) Clear()                                            { c.queue.clear() }
func (c *Console) Stop()                                             { c.cancel(); c.queue.close() }
