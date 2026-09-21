package drissionpage

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// ResponseHook runs after HTTP headers arrive, before the body is consumed. It
// may inspect or replace Body; it owns closing the old Body when replacing it.
// Hooks apply to ordinary requests and Download, including redirect responses.
type ResponseHook func(*http.Response) error
type sessionTransport struct {
	base   http.RoundTripper
	mounts map[string]http.RoundTripper
	hooks  []ResponseHook
}

func (s *sessionTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	transport := s.base
	prefixes := make([]string, 0, len(s.mounts))
	for prefix := range s.mounts {
		prefixes = append(prefixes, prefix)
	}
	sort.Slice(prefixes, func(i, j int) bool { return len(prefixes[i]) > len(prefixes[j]) })
	for _, prefix := range prefixes {
		if strings.HasPrefix(r.URL.String(), prefix) {
			transport = s.mounts[prefix]
			break
		}
	}
	response, err := transport.RoundTrip(r)
	if err != nil {
		return response, err
	}
	for _, hook := range s.hooks {
		if err = hook(response); err != nil {
			if response.Body != nil {
				response.Body.Close()
			}
			return nil, err
		}
	}
	return response, nil
}
func (s *sessionTransport) CloseIdleConnections() {
	if closer, ok := s.base.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
	for _, transport := range s.mounts {
		if closer, ok := transport.(interface{ CloseIdleConnections() }); ok {
			closer.CloseIdleConnections()
		}
	}
}
func (p *SessionPage) changeExtensions(change func(*sessionTransport) error) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrClosed
	}
	next := &sessionTransport{base: p.client.Transport, mounts: map[string]http.RoundTripper{}}
	if old, ok := next.base.(*sessionTransport); ok {
		next.base = old.base
		next.hooks = append([]ResponseHook(nil), old.hooks...)
		for key, value := range old.mounts {
			next.mounts[key] = value
		}
	}
	if next.base == nil {
		next.base = http.DefaultTransport
	}
	if err := change(next); err != nil {
		return err
	}
	client := *p.client
	client.Transport = next
	p.client = &client
	return nil
}

// Mount selects an adapter by longest URL prefix. A nil adapter removes a mount.
func (p *SessionPage) Mount(prefix string, adapter http.RoundTripper) error {
	if prefix == "" {
		return fmt.Errorf("adapter prefix must not be empty")
	}
	return p.changeExtensions(func(next *sessionTransport) error {
		if adapter == nil {
			delete(next.mounts, prefix)
		} else {
			next.mounts[prefix] = adapter
		}
		return nil
	})
}
func (p *SessionPage) SetResponseHooks(hooks ...ResponseHook) error {
	for _, hook := range hooks {
		if hook == nil {
			return fmt.Errorf("nil response hook")
		}
	}
	return p.changeExtensions(func(next *sessionTransport) error { next.hooks = append([]ResponseHook(nil), hooks...); return nil })
}
