package drissionpage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	// MaxActiveRequests bounds retained request and extra-info metadata; zero uses 4096.
	// Exceeding it stops capture with ErrListenerCapacity.
	MaxActiveRequests int
}
type DataPacket struct {
	TabID         string
	RequestID     string
	FrameID       string
	PostDataError error
	URL           string
	Method        string
	ResourceType  string
	Request       *proto.NetworkRequest
	Response      *proto.NetworkResponse
	Body          []byte
	Failure       string
	BodyError     error
	Timestamp     time.Time
	Extras        *PacketExtras
}

func (p *DataPacket) JSON(out any) error { return json.Unmarshal(p.Body, out) }

type StreamMessage struct {
	TabID                                          string
	RequestID, URL, Kind, EventName, EventID, Data string
	Opcode                                         float64
	Timestamp                                      time.Time
	FrameID                                        string
	HandshakeRequest                               *proto.NetworkWebSocketRequest
	HandshakeResponse                              *proto.NetworkWebSocketResponse
}
type Listener struct {
	packets  *eventQueue[*DataPacket]
	streams  *eventQueue[*StreamMessage]
	cancel   context.CancelFunc
	paused   atomic.Bool
	active   atomic.Int64
	overflow atomic.Bool
	matcher  atomic.Pointer[networkMatch]
}

func (t *ChromiumTab) Listen(ctx context.Context, filter ListenFilter) (*Listener, error) {
	filter = cloneListenFilter(filter)
	compiled, err := compileNetworkFilter(filter)
	if err != nil {
		return nil, err
	}
	life, cancel := context.WithCancel(ctx)
	p := t.page.Context(life)
	l := &Listener{packets: newEventQueue[*DataPacket](), streams: newEventQueue[*StreamMessage](), cancel: cancel}
	l.matcher.Store(&networkMatch{filter: filter, match: compiled})
	matches := func(target, method, kind string) bool { return l.matcher.Load().match(target, method, kind) }
	l.packets.setLimit(filter.BufferSize)
	l.streams.setLimit(filter.BufferSize)
	limit := filter.MaxActiveRequests
	if limit <= 0 {
		limit = 4096
	}
	pending := map[proto.NetworkRequestID]*DataPacket{}
	extras := map[proto.NetworkRequestID]*extraChain{}
	getChain := func(id proto.NetworkRequestID) *extraChain {
		x := extras[id]
		if x == nil {
			if len(extras) >= limit {
				l.overflow.Store(true)
				cancel()
				return &extraChain{}
			}
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
	type socketInfo struct {
		url      string
		request  *proto.NetworkWebSocketRequest
		response *proto.NetworkWebSocketResponse
	}
	sockets := map[proto.NetworkRequestID]*socketInfo{}
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
			if len(chain.hops) >= limit {
				l.overflow.Store(true)
				cancel()
				return
			}
			packetExtras := chain.add()
			if life.Err() != nil {
				return
			}
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
			pending[e.RequestID] = &DataPacket{RequestID: string(e.RequestID), FrameID: string(e.FrameID), URL: e.Request.URL, Method: e.Request.Method, ResourceType: string(e.Type), Request: e.Request, Timestamp: time.Now(), Extras: packetExtras}
			if e.Request.HasPostData && e.Request.PostData == "" {
				body, err := (proto.NetworkGetRequestPostData{RequestID: e.RequestID}).Call(p)
				if err != nil {
					pending[e.RequestID].PostDataError = err
				} else {
					copy := *e.Request
					copy.PostData = body.PostData
					pending[e.RequestID].Request = &copy
				}
			}
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
			if len(x.requests) >= limit {
				l.overflow.Store(true)
				cancel()
				return
			}
			x.requests = append(x.requests, e)
			x.drain()
			clean(e.RequestID)
		},
		func(e *proto.NetworkResponseReceivedExtraInfo) {
			x := getChain(e.RequestID)
			if len(x.responses) >= limit {
				l.overflow.Store(true)
				cancel()
				return
			}
			x.responses = append(x.responses, e)
			x.drain()
			clean(e.RequestID)
		},
		func(e *proto.NetworkLoadingFinished) { finish(e.RequestID, "") },
		func(e *proto.NetworkLoadingFailed) { finish(e.RequestID, e.ErrorText) },
		func(e *proto.NetworkWebSocketCreated) {
			if len(sockets) >= limit {
				l.overflow.Store(true)
				cancel()
				return
			}
			sockets[e.RequestID] = &socketInfo{url: e.URL}
		},
		func(e *proto.NetworkWebSocketWillSendHandshakeRequest) {
			if info := sockets[e.RequestID]; info != nil {
				info.request = e.Request
			}
		},
		func(e *proto.NetworkWebSocketHandshakeResponseReceived) {
			if info := sockets[e.RequestID]; info != nil {
				info.response = e.Response
			}
		},
		func(e *proto.NetworkWebSocketFrameError) {
			if info := sockets[e.RequestID]; info != nil && !l.paused.Load() && matches(info.url, "GET", "WebSocket") {
				l.streams.push(&StreamMessage{RequestID: string(e.RequestID), URL: info.url, Kind: "websocket-error", Data: e.ErrorMessage, Timestamp: time.Now(), FrameID: string(p.FrameID), HandshakeRequest: info.request, HandshakeResponse: info.response})
			}
		},
		func(e *proto.NetworkWebSocketFrameReceived) {
			info := sockets[e.RequestID]
			if info == nil {
				return
			}
			u := info.url
			if !l.paused.Load() && matches(u, "GET", "WebSocket") {
				l.streams.push(&StreamMessage{RequestID: string(e.RequestID), URL: u, Kind: "websocket-received", FrameID: string(p.FrameID), HandshakeRequest: info.request, HandshakeResponse: info.response, Data: e.Response.PayloadData, Opcode: e.Response.Opcode, Timestamp: time.Now()})
			}
		},
		func(e *proto.NetworkWebSocketFrameSent) {
			info := sockets[e.RequestID]
			if info == nil {
				return
			}
			u := info.url
			if !l.paused.Load() && matches(u, "GET", "WebSocket") {
				l.streams.push(&StreamMessage{RequestID: string(e.RequestID), URL: u, Kind: "websocket-sent", FrameID: string(p.FrameID), HandshakeRequest: info.request, HandshakeResponse: info.response, Data: e.Response.PayloadData, Opcode: e.Response.Opcode, Timestamp: time.Now()})
			}
		},
		func(e *proto.NetworkWebSocketClosed) { delete(sockets, e.RequestID) },
		func(e *proto.NetworkEventSourceMessageReceived) {
			if packet := pending[e.RequestID]; packet != nil && !l.paused.Load() {
				l.streams.push(&StreamMessage{RequestID: string(e.RequestID), URL: packet.URL, Kind: "sse", FrameID: packet.FrameID, EventName: e.EventName, EventID: e.EventID, Data: e.Data, Timestamp: time.Now()})
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
		p, e := l.Next(ctx)
		if e != nil {
			return out, e
		}
		out = append(out, p)
	}
	return out, nil
}

var ErrListenerCapacity = fmt.Errorf("listener active metadata capacity exceeded")

func (l *Listener) Err() error {
	if l.overflow.Load() {
		return ErrListenerCapacity
	}
	return nil
}
func (l *Listener) Next(ctx context.Context) (*DataPacket, error) {
	packet, err := l.packets.pop(ctx)
	if err != nil && l.Err() != nil {
		err = l.Err()
	}
	return packet, err
}
func (l *Listener) NextStream(ctx context.Context) (*StreamMessage, error) {
	message, err := l.streams.pop(ctx)
	if err != nil && l.Err() != nil {
		err = l.Err()
	}
	return message, err
}
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
		if err := l.Err(); err != nil {
			return err
		}
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

type networkMatch struct {
	filter ListenFilter
	match  func(string, string, string) bool
}

func compileNetworkFilter(filter ListenFilter) (func(string, string, string) bool, error) {
	filter.URLs = append([]string(nil), filter.URLs...)
	filter.Methods = append([]string(nil), filter.Methods...)
	filter.ResourceTypes = append([]string(nil), filter.ResourceTypes...)
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
	return matches, nil
}

// SetFilter updates matching for future requests. Already-started requests keep
// their original match. Queue/capacity limits are fixed at Listen creation.
func (l *Listener) SetFilter(filter ListenFilter) error {
	filter = cloneListenFilter(filter)
	match, err := compileNetworkFilter(filter)
	if err != nil {
		return err
	}
	l.matcher.Store(&networkMatch{filter: filter, match: match})
	return nil
}
func (l *Listener) SetURLs(regex bool, urls ...string) error {
	filter := l.matcher.Load().filter
	filter.URLs = urls
	filter.Regex = regex
	return l.SetFilter(filter)
}
func (l *Listener) SetMethods(methods ...string) error {
	filter := l.matcher.Load().filter
	filter.Methods = methods
	return l.SetFilter(filter)
}
func (l *Listener) SetResourceTypes(types ...string) error {
	filter := l.matcher.Load().filter
	filter.ResourceTypes = types
	return l.SetFilter(filter)
}

func (c *Console) Messages() []*ConsoleMessage { return c.queue.snapshot() }
func (c *Console) Wait(ctx context.Context, count int) ([]*ConsoleMessage, error) {
	messages := []*ConsoleMessage{}
	for len(messages) < count {
		message, err := c.Next(ctx)
		if err != nil {
			return messages, err
		}
		messages = append(messages, message)
	}
	return messages, nil
}

func cloneListenFilter(f ListenFilter) ListenFilter {
	f.URLs = append([]string(nil), f.URLs...)
	f.Methods = append([]string(nil), f.Methods...)
	f.ResourceTypes = append([]string(nil), f.ResourceTypes...)
	return f
}
