package drissionpage

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/publicsuffix"
)

type trackedJar struct {
	jar *cookiejar.Jar
	mu  sync.Mutex
	all map[string]*http.Cookie
}

func newTrackedJar() (*trackedJar, error) {
	jar, e := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if e != nil {
		return nil, e
	}
	return &trackedJar{jar: jar, all: map[string]*http.Cookie{}}, nil
}
func (j *trackedJar) Cookies(u *url.URL) []*http.Cookie { return j.jar.Cookies(u) }
func (j *trackedJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.jar.SetCookies(u, cookies)
	host := strings.ToLower(u.Hostname())
	for _, c := range cookies {
		copy := *c
		domain := strings.ToLower(strings.TrimPrefix(c.Domain, "."))
		if domain == "" {
			domain = host
		}
		if host != domain && !strings.HasSuffix(host, "."+domain) {
			continue
		}
		if suffix, _ := publicsuffix.PublicSuffix(domain); suffix == domain && domain != host {
			continue
		}
		copy.Domain = domain
		if copy.Path == "" || copy.Path[0] != '/' {
			copy.Path = "/"
			if i := strings.LastIndex(u.Path, "/"); i > 0 {
				copy.Path = u.Path[:i]
			}
		}
		key := domain + "\n" + copy.Path + "\n" + copy.Name
		if copy.MaxAge < 0 || (!copy.Expires.IsZero() && !copy.Expires.After(time.Now())) {
			delete(j.all, key)
			continue
		}
		if copy.MaxAge > 0 {
			copy.Expires = time.Now().Add(time.Duration(copy.MaxAge) * time.Second)
		}
		j.all[key] = &copy
	}
}
func (j *trackedJar) snapshot() []*http.Cookie {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := make([]*http.Cookie, 0, len(j.all))
	for key, c := range j.all {
		if !c.Expires.IsZero() && !c.Expires.After(time.Now()) {
			delete(j.all, key)
			continue
		}
		copy := *c
		out = append(out, &copy)
	}
	return out
}
func (p *SessionPage) AllCookies() ([]*http.Cookie, error) {
	j, ok := p.Client().Jar.(*trackedJar)
	if !ok {
		return nil, fmt.Errorf("custom cookie jar does not expose all-domain metadata")
	}
	return j.snapshot(), nil
}
