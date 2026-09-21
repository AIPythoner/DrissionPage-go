package drissionpage

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"golang.org/x/net/html"
)

type Axis string

const (
	Ancestor        Axis = "ancestor"
	Child           Axis = "child"
	PreviousSibling Axis = "preceding-sibling"
	NextSibling     Axis = "following-sibling"
	Before          Axis = "preceding"
	After           Axis = "following"
)

func axisQuery(axis Axis) (string, error) {
	switch axis {
	case Ancestor, Child, PreviousSibling, NextSibling, Before, After:
		return string(axis) + "::*", nil
	}
	return "", fmt.Errorf("invalid axis %q", axis)
}
func (e *SessionElement) Relatives(axis Axis) ([]*SessionElement, error) {
	q, err := axisQuery(axis)
	if err != nil {
		return nil, err
	}
	els, err := e.Eles(XPath(q))
	if axis == Ancestor || axis == PreviousSibling || axis == Before {
		slices.Reverse(els)
	}
	return els, err
}
func (e *ChromiumElement) Relatives(ctx context.Context, axis Axis) ([]*ChromiumElement, error) {
	q, err := axisQuery(axis)
	if err != nil {
		return nil, err
	}
	els, err := e.Eles(ctx, XPath(q))
	if axis == Ancestor || axis == PreviousSibling || axis == Before {
		slices.Reverse(els)
	}
	return els, err
}
func (e *SessionElement) Relative(axis Axis, index int, filter ElementFilter) (*SessionElement, error) {
	els, err := e.Relatives(axis)
	if err != nil {
		return nil, err
	}
	return selectIndex(FilterSession(els, filter), index)
}
func (e *ChromiumElement) Relative(ctx context.Context, axis Axis, index int, filter ElementFilter) (*ChromiumElement, error) {
	els, err := e.Relatives(ctx, axis)
	if err != nil {
		return nil, err
	}
	els, err = FilterChromium(ctx, els, filter)
	if err != nil {
		return nil, err
	}
	return selectIndex(els, index)
}
func (e *SessionElement) Path() string {
	var parts []string
	for n := e.node; n != nil && n.Type == html.ElementNode; n = n.Parent {
		i := 1
		for s := n.PrevSibling; s != nil; s = s.PrevSibling {
			if s.Type == html.ElementNode && s.Data == n.Data {
				i++
			}
		}
		parts = append([]string{fmt.Sprintf("%s[%d]", n.Data, i)}, parts...)
	}
	return "/" + strings.Join(parts, "/")
}
func (e *SessionElement) CSSPath() string {
	var parts []string
	for n := e.node; n != nil && n.Type == html.ElementNode; n = n.Parent {
		i := 1
		for s := n.PrevSibling; s != nil; s = s.PrevSibling {
			if s.Type == html.ElementNode && s.Data == n.Data {
				i++
			}
		}
		parts = append([]string{fmt.Sprintf("%s:nth-of-type(%d)", n.Data, i)}, parts...)
	}
	return strings.Join(parts, " > ")
}
func (e *SessionElement) Tree() string {
	var out strings.Builder
	var walk func(*html.Node, int)
	walk = func(n *html.Node, depth int) {
		if n.Type == html.ElementNode {
			out.WriteString(strings.Repeat("  ", depth))
			out.WriteString(n.Data)
			for _, a := range n.Attr {
				fmt.Fprintf(&out, " %s=%q", a.Key, a.Val)
			}
			out.WriteByte('\n')
			depth++
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, depth)
		}
	}
	walk(e.node, 0)
	return out.String()
}
func (e *ChromiumElement) Tree(ctx context.Context) (string, error) {
	s, err := e.Snapshot(ctx)
	if err != nil {
		return "", err
	}
	return s.Tree(), nil
}

type ElementFilter struct {
	Tag                                              string
	Text                                             string
	ExactText                                        bool
	Attributes                                       map[string]string
	Displayed, Checked, Selected, Enabled, Clickable *bool
}

func FilterSession(els []*SessionElement, f ElementFilter) []*SessionElement {
	out := make([]*SessionElement, 0)
	for _, e := range els {
		if f.Tag != "" && e.Tag() != f.Tag {
			continue
		}
		text := e.RawText()
		if f.Text != "" && ((f.ExactText && text != f.Text) || (!f.ExactText && !strings.Contains(text, f.Text))) {
			continue
		}
		ok := true
		for k, v := range f.Attributes {
			value, exists := e.Attr(k)
			if !exists || value != v {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, e)
		}
	}
	return out
}
func FilterChromium(ctx context.Context, els []*ChromiumElement, f ElementFilter) ([]*ChromiumElement, error) {
	out := make([]*ChromiumElement, 0)
	for _, e := range els {
		if f.Tag != "" {
			tag, err := e.Tag(ctx)
			if err != nil {
				return nil, err
			}
			if tag != f.Tag {
				continue
			}
		}
		if f.Text != "" {
			text, err := e.RawText(ctx)
			if err != nil {
				return nil, err
			}
			if (f.ExactText && text != f.Text) || (!f.ExactText && !strings.Contains(text, f.Text)) {
				continue
			}
		}
		ok := true
		for k, v := range f.Attributes {
			value, exists, err := e.Attr(ctx, k)
			if err != nil {
				return nil, err
			}
			if !exists || value != v {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		if f.Displayed != nil || f.Checked != nil || f.Selected != nil || f.Enabled != nil || f.Clickable != nil {
			s, err := e.States(ctx)
			if err != nil {
				return nil, err
			}
			for _, pair := range []struct {
				want *bool
				got  bool
			}{{f.Displayed, s.Displayed}, {f.Checked, s.Checked}, {f.Selected, s.Selected}, {f.Enabled, s.Enabled}, {f.Clickable, s.Clickable}} {
				if pair.want != nil && *pair.want != pair.got {
					ok = false
				}
			}
		}
		if ok {
			out = append(out, e)
		}
	}
	return out, nil
}
