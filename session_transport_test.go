package drissionpage

import (
	"context"
	"encoding/json"
	"golang.org/x/text/encoding/simplifiedchinese"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestSessionTransportAndEncoding(t *testing.T) {
	text := "编码测试"
	gbk, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gbk":
			w.Header().Set("Content-Type", "text/plain")
			w.Write(gbk)
		case "/redirect":
			http.Redirect(w, r, "/echo", 302)
		default:
			user, pass, _ := r.BasicAuth()
			json.NewEncoder(w).Encode(map[string]string{"q": r.URL.Query().Get("q"), "user": user, "pass": pass})
		}
	}))
	defer server.Close()
	p, err := NewSessionPage()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx := context.Background()
	if err = p.SetParams(url.Values{"q": {"default"}}); err != nil {
		t.Fatal(err)
	}
	if err = p.SetAuth("user", "pass"); err != nil {
		t.Fatal(err)
	}
	r, err := p.Get(ctx, server.URL+"/echo", RequestOptions{Params: url.Values{"q": {"override"}}})
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]string
	if err = r.JSON(&data); err != nil {
		t.Fatal(err)
	}
	if data["q"] != "override" || data["user"] != "user" || data["pass"] != "pass" {
		t.Fatal(data)
	}
	if _, err = p.Get(ctx, server.URL+"/gbk"); err != nil {
		t.Fatal(err)
	}
	if err = p.SetEncoding("gbk", true); err != nil {
		t.Fatal(err)
	}
	r, err = p.Response()
	if err != nil || r.Text != text {
		t.Fatal(r, err)
	}
	r, err = p.Get(ctx, server.URL+"/gbk")
	if err != nil || r.Text != text {
		t.Fatal(r, err)
	}
	if p.SetEncoding("unknown-character-set", true) == nil {
		t.Fatal("invalid encoding accepted")
	}
	if err = p.SetMaxRedirects(0); err != nil {
		t.Fatal(err)
	}
	if _, err = p.Get(ctx, server.URL+"/redirect"); err == nil {
		t.Fatal("redirect limit ignored")
	}
	if err = p.SetMaxRedirects(1); err != nil {
		t.Fatal(err)
	}
	if _, err = p.Get(ctx, server.URL+"/redirect"); err != nil {
		t.Fatal(err)
	}
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "proxy-response") }))
	defer proxy.Close()
	if err = p.SetProxies(proxy.URL, proxy.URL); err != nil {
		t.Fatal(err)
	}
	r, err = p.Get(ctx, "http://unreachable.invalid/echo")
	if err != nil || r.Text != "proxy-response" {
		t.Fatal(r, err)
	}
	if err = p.SetProxies("", ""); err != nil {
		t.Fatal(err)
	}
	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "tls-ok") }))
	defer tlsServer.Close()
	if _, err = p.Get(ctx, tlsServer.URL); err == nil {
		t.Fatal("untrusted TLS accepted")
	}
	if err = p.SetVerifyTLS(false); err != nil {
		t.Fatal(err)
	}
	r, err = p.Get(ctx, tlsServer.URL)
	if err != nil || r.Text != "tls-ok" {
		t.Fatal(r, err)
	}
	if err = p.SetVerifyTLS(true); err != nil {
		t.Fatal(err)
	}
	if _, err = p.Get(ctx, tlsServer.URL); err == nil {
		t.Fatal("TLS verification not restored")
	}
}
