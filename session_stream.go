package drissionpage

import (
	"context"
	"net/http"
	"strings"
)

// OpenStream returns headers immediately and leaves Body reading/closing to the
// caller. It does not buffer the response or replace the page's parsed document.
// Retries stop once response headers have been returned to the caller.
func (p *SessionPage) OpenStream(ctx context.Context, method, target string, options ...RequestOptions) (*http.Response, error) {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, ErrClosed
	}
	settings := p.options
	settings.Headers = p.options.Headers.Clone()
	settings.Params = cloneValues(p.options.Params)
	client := *p.client
	if p.requestTimeout != nil {
		client.Timeout = *p.requestTimeout
	}
	p.mu.RUnlock()
	option := RequestOptions{}
	if len(options) > 0 {
		option = options[0]
	}
	method = strings.ToUpper(method)
	retries := settings.RetryTimes
	if method != "GET" && method != "HEAD" && method != "OPTIONS" && !option.RetryUnsafe {
		retries = 0
	}
	var err error
	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			if err = waitDuration(ctx, settings.RetryInterval); err != nil {
				return nil, err
			}
		}
		request, e := buildSessionRequest(ctx, method, target, settings, option)
		if e != nil {
			return nil, e
		}
		response, e := client.Do(request)
		if e == nil {
			return response, nil
		}
		err = e
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return nil, err
}
