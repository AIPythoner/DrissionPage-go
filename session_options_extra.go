package drissionpage

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"os"
)

func (o *SessionOptions) SetHeaders(headers http.Header) *SessionOptions {
	o.Headers = headers.Clone()
	return o
}
func (o *SessionOptions) RemoveHeader(name string) *SessionOptions { o.Headers.Del(name); return o }
func (o *SessionOptions) ClearHeaders() *SessionOptions            { o.Headers = http.Header{}; return o }
func (o *SessionOptions) SetParams(params url.Values) *SessionOptions {
	o.Params = cloneValues(params)
	return o
}
func (o *SessionOptions) SetAuth(username, password string) *SessionOptions {
	o.Username = username
	o.Password = password
	return o
}
func (o *SessionOptions) SetProxies(httpProxy, httpsProxy string) *SessionOptions {
	o.HTTPProxy = httpProxy
	o.HTTPSProxy = httpsProxy
	o.Proxy = ""
	return o
}
func (o *SessionOptions) SetVerify(on bool) *SessionOptions   { o.VerifyTLS = &on; return o }
func (o *SessionOptions) SetTrustEnv(on bool) *SessionOptions { o.TrustEnv = &on; return o }
func (o *SessionOptions) SetMaxRedirects(count int) *SessionOptions {
	o.MaxRedirects = &count
	return o
}
func (o *SessionOptions) SetResponseHooks(hooks ...ResponseHook) *SessionOptions {
	o.Hooks = append([]ResponseHook(nil), hooks...)
	return o
}
func (o *SessionOptions) AddAdapter(prefix string, adapter http.RoundTripper) *SessionOptions {
	if o.Adapters == nil {
		o.Adapters = map[string]http.RoundTripper{}
	}
	o.Adapters[prefix] = adapter
	return o
}
func (o *SessionOptions) SetClientCertificate(cert, key string) *SessionOptions {
	o.CertFile = cert
	o.KeyFile = key
	return o
}

func (p *SessionPage) applyOptions(o SessionOptions) error {
	if o.TrustEnv != nil {
		if err := p.SetTrustEnv(*o.TrustEnv); err != nil {
			return err
		}
	}
	if o.HTTPProxy != "" || o.HTTPSProxy != "" {
		if err := p.SetProxies(o.HTTPProxy, o.HTTPSProxy); err != nil {
			return err
		}
	}
	if o.VerifyTLS != nil {
		if err := p.SetVerifyTLS(*o.VerifyTLS); err != nil {
			return err
		}
	}
	if o.CAFile != "" {
		data, err := os.ReadFile(o.CAFile)
		if err != nil {
			return err
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(data) {
			return fmt.Errorf("CA file contains no valid certificates")
		}
		if err = p.configureTransport(func(transport *http.Transport) {
			if transport.TLSClientConfig == nil {
				transport.TLSClientConfig = &tls.Config{}
			}
			transport.TLSClientConfig.RootCAs = pool
		}); err != nil {
			return err
		}
	}
	if o.CertFile != "" {
		if err := p.SetClientCertificate(o.CertFile, o.KeyFile); err != nil {
			return err
		}
	}
	if o.MaxRedirects != nil {
		if err := p.SetMaxRedirects(*o.MaxRedirects); err != nil {
			return err
		}
	}
	for prefix, adapter := range o.Adapters {
		if err := p.Mount(prefix, adapter); err != nil {
			return err
		}
	}
	if len(o.Hooks) > 0 {
		return p.SetResponseHooks(o.Hooks...)
	}
	return nil
}
