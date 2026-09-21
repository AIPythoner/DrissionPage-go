package drissionpage

import (
	"fmt"
	"net/http"
	"time"
)

// SetHeaders replaces default headers. Request-specific headers take precedence.
func (p *SessionPage) SetHeaders(headers http.Header) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrClosed
	}
	p.options.Headers = make(http.Header)
	for key, values := range headers {
		for _, value := range values {
			p.options.Headers.Add(key, value)
		}
	}
	return nil
}

func (p *SessionPage) SetHeader(name, value string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrClosed
	}
	if p.options.Headers == nil {
		p.options.Headers = make(http.Header)
	}
	p.options.Headers.Set(name, value)
	return nil
}

func (p *SessionPage) SetUserAgent(value string) error { return p.SetHeader("User-Agent", value) }

// SetTimeout affects subsequent requests; zero disables the request timeout.
func (p *SessionPage) SetTimeout(timeout time.Duration) error {
	if timeout < 0 {
		return fmt.Errorf("negative timeout")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrClosed
	}
	p.options.Timeout = timeout
	p.requestTimeout = &timeout
	return nil
}

func (p *SessionPage) SetRetry(times int, interval time.Duration) error {
	if times < 0 || interval < 0 {
		return fmt.Errorf("negative retry count or interval")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrClosed
	}
	p.options.RetryTimes, p.options.RetryInterval = times, interval
	return nil
}
