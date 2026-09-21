package drissionpage

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/AIPythoner/DrissionPage-go/internal/htmlquery"
	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
)

type SessionElement struct {
	node    *html.Node
	baseURL string
}

func MakeSessionElement(markup string, baseURL ...string) (*SessionElement, error) {
	n, e := htmlquery.Parse(strings.NewReader(markup))
	if e != nil {
		return nil, e
	}
	base := ""
	if len(baseURL) > 0 {
		base = baseURL[0]
	}
	return &SessionElement{n, base}, nil
}
func (e *SessionElement) Node() *html.Node  { return e.node }
func (e *SessionElement) Tag() string       { return e.node.Data }
func (e *SessionElement) Text() string      { return formattedText(e.node) }
func (e *SessionElement) RawText() string   { return htmlquery.InnerText(e.node) }
func (e *SessionElement) HTML() string      { return htmlquery.OutputHTML(e.node, true) }
func (e *SessionElement) InnerHTML() string { return htmlquery.OutputHTML(e.node, false) }
func (e *SessionElement) Attrs() map[string]string {
	out := map[string]string{}
	for _, a := range e.node.Attr {
		out[a.Key] = a.Val
	}
	return out
}
func (e *SessionElement) Attr(name string) (string, bool) {
	if name == "text" {
		return e.Text(), true
	}
	if name == "innerText" {
		return e.RawText(), true
	}
	if name == "html" {
		return e.HTML(), true
	}
	if name == "innerHTML" {
		return e.InnerHTML(), true
	}
	for _, a := range e.node.Attr {
		if a.Key == name {
			v := a.Val
			if name == "href" || name == "src" {
				if base, err := url.Parse(e.baseURL); err == nil {
					if ref, err := url.Parse(v); err == nil {
						v = base.ResolveReference(ref).String()
					}
				}
			}
			return v, true
		}
	}
	return "", false
}
func (e *SessionElement) Eles(value any) ([]*SessionElement, error) {
	loc, err := ParseLocator(value)
	if err != nil {
		return nil, err
	}
	if loc.Kind == "ax" {
		return nil, fmt.Errorf("%w: accessibility requires a browser", ErrInvalidLocator)
	}
	var nodes []*html.Node
	if loc.Kind == "search" {
		if strings.HasPrefix(loc.Value, "#") || strings.HasPrefix(loc.Value, ".") {
			loc = CSS(loc.Value)
		} else {
			loc = XPath(".//*/text()[contains(.," + xpathLiteral(loc.Value) + ")]/..")
		}
	}
	if loc.Kind == "css" {
		sel, err := cascadia.Compile(loc.Value)
		if err != nil {
			return nil, err
		}
		nodes = cascadia.QueryAll(e.node, sel)
	} else {
		q := loc.Value
		if strings.HasPrefix(q, "//") {
			q = "." + q
		}
		nodes, err = htmlquery.QueryAll(e.node, q)
		if err != nil {
			return nil, err
		}
	}
	out := make([]*SessionElement, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, &SessionElement{n, e.baseURL})
	}
	return out, nil
}
func (e *SessionElement) Ele(value any, index ...int) (*SessionElement, error) {
	i := 1
	if len(index) > 0 {
		i = index[0]
	}
	nodes, err := e.Eles(value)
	if err != nil {
		return nil, err
	}
	return selectIndex(nodes, i)
}
func (e *SessionElement) Parent() (*SessionElement, error) {
	if e.node.Parent == nil {
		return nil, ErrElementNotFound
	}
	return &SessionElement{e.node.Parent, e.baseURL}, nil
}
func (e *SessionElement) Children() []*SessionElement {
	var out []*SessionElement
	for n := e.node.FirstChild; n != nil; n = n.NextSibling {
		if n.Type == html.ElementNode {
			out = append(out, &SessionElement{n, e.baseURL})
		}
	}
	return out
}
func (e *SessionElement) Next() (*SessionElement, error) {
	for n := e.node.NextSibling; n != nil; n = n.NextSibling {
		if n.Type == html.ElementNode {
			return &SessionElement{n, e.baseURL}, nil
		}
	}
	return nil, ErrElementNotFound
}
func (e *SessionElement) Prev() (*SessionElement, error) {
	for n := e.node.PrevSibling; n != nil; n = n.PrevSibling {
		if n.Type == html.ElementNode {
			return &SessionElement{n, e.baseURL}, nil
		}
	}
	return nil, ErrElementNotFound
}
