package drissionpage

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"golang.org/x/net/html/charset"
	"io"
	"net/http"
	"net/url"
)

func cloneValues(v url.Values) url.Values {
	out := make(url.Values, len(v))
	for k, vs := range v {
		out[k] = append([]string(nil), vs...)
	}
	return out
}
func (p *SessionPage) SetParams(params url.Values) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrClosed
	}
	p.options.Params = cloneValues(params)
	return nil
}
func (p *SessionPage) SetAuth(username, password string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrClosed
	}
	p.options.Username = username
	p.options.Password = password
	return nil
}

// SetEncoding changes decoding of the current response. all also changes future
// requests; an empty label restores automatic decoding.
func (p *SessionPage) SetEncoding(label string, all bool) error {
	if label != "" {
		if encoding, _ := charset.Lookup(label); encoding == nil {
			return fmt.Errorf("unknown encoding %q", label)
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrClosed
	}
	if p.response != nil {
		var reader io.Reader
		var err error
		if label == "" {
			reader, err = charset.NewReader(bytes.NewReader(p.response.Body), p.response.Headers.Get("Content-Type"))
		} else {
			reader, err = charset.NewReaderLabel(label, bytes.NewReader(p.response.Body))
		}
		if err != nil {
			return err
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			return err
		}
		document, err := MakeSessionElement(string(data), p.response.URL)
		if err != nil {
			return err
		}
		copy := *p.response
		copy.Text = string(data)
		p.response = &copy
		p.document = document
	}
	if all {
		p.options.Encoding = label
	}
	return nil
}

func (p *SessionPage) configureTransport(change func(*http.Transport)) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrClosed
	}
	transport := p.client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	original, ok := transport.(*http.Transport)
	if !ok {
		return fmt.Errorf("custom RoundTripper cannot be reconfigured; supply a new HTTP client")
	}
	next := original.Clone()
	change(next)
	client := *p.client
	client.Transport = next
	p.client = &client
	original.CloseIdleConnections()
	return nil
}

// SetProxies configures each URL scheme independently. Empty means direct.
func (p *SessionPage) SetProxies(httpProxy, httpsProxy string) error {
	proxies := map[string]*url.URL{}
	for scheme, value := range map[string]string{"http": httpProxy, "https": httpsProxy} {
		if value == "" {
			continue
		}
		u, err := url.Parse(value)
		if err != nil {
			return err
		}
		if u.Host == "" || u.Scheme == "" {
			return fmt.Errorf("invalid proxy URL")
		}
		proxies[scheme] = u
	}
	return p.configureTransport(func(t *http.Transport) {
		t.Proxy = func(r *http.Request) (*url.URL, error) { return proxies[r.URL.Scheme], nil }
	})
}
func (p *SessionPage) SetTrustEnv(enabled bool) error {
	return p.configureTransport(func(t *http.Transport) {
		if enabled {
			t.Proxy = http.ProxyFromEnvironment
		} else {
			t.Proxy = nil
		}
	})
}
func (p *SessionPage) SetTLSConfig(config *tls.Config) error {
	return p.configureTransport(func(t *http.Transport) {
		if config == nil {
			t.TLSClientConfig = nil
		} else {
			t.TLSClientConfig = config.Clone()
		}
	})
}
func (p *SessionPage) SetVerifyTLS(enabled bool) error {
	return p.configureTransport(func(t *http.Transport) {
		if t.TLSClientConfig == nil {
			t.TLSClientConfig = &tls.Config{}
		}
		t.TLSClientConfig.InsecureSkipVerify = !enabled
	})
}
func (p *SessionPage) SetClientCertificate(certFile, keyFile string) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}
	return p.configureTransport(func(t *http.Transport) {
		if t.TLSClientConfig == nil {
			t.TLSClientConfig = &tls.Config{}
		}
		t.TLSClientConfig.Certificates = []tls.Certificate{cert}
	})
}

// SetMaxRedirects replaces the redirect callback; zero rejects the first redirect.
func (p *SessionPage) SetMaxRedirects(limit int) error {
	if limit < 0 {
		return fmt.Errorf("negative redirect limit")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrClosed
	}
	client := *p.client
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > limit {
			return fmt.Errorf("redirect limit %d exceeded", limit)
		}
		return nil
	}
	p.client = &client
	return nil
}
