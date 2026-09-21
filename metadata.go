package drissionpage

import (
	"context"
	"fmt"
	"github.com/go-rod/rod/lib/proto"
	"golang.org/x/net/html"
	"strings"
	"time"
)

func (e *SessionElement) Link() (string, bool) {
	if value, ok := e.Attr("href"); ok && value != "" {
		return value, true
	}
	return e.Attr("src")
}
func (e *ChromiumElement) Link(ctx context.Context) (string, bool, error) {
	value, ok, err := e.Attr(ctx, "href")
	if err != nil || ok && value != "" {
		return value, ok, err
	}
	return e.Attr(ctx, "src")
}
func (e *SessionElement) ChildCount() int { return len(e.Children()) }
func (e *ChromiumElement) ChildCount(ctx context.Context) (int, error) {
	children, err := e.Children(ctx)
	return len(children), err
}
func (e *SessionElement) Comments() []string {
	comments := []string{}
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.CommentNode {
				comments = append(comments, child.Data)
			}
			walk(child)
		}
	}
	walk(e.node)
	return comments
}
func (e *ChromiumElement) Comments(ctx context.Context) ([]string, error) {
	snapshot, err := e.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	return snapshot.Comments(), nil
}
func (e *SessionElement) Texts(textOnly bool) []string {
	texts := []string{}
	for child := e.node.FirstChild; child != nil; child = child.NextSibling {
		value := ""
		if child.Type == html.TextNode {
			value = child.Data
		} else if !textOnly && child.Type == html.ElementNode {
			value = formattedText(child)
		}
		if strings.Trim(value, "\r\n\t ") != "" {
			texts = append(texts, FormatHTML(strings.TrimRight(strings.Trim(value, " "), "\n")))
		}
	}
	return texts
}
func (e *ChromiumElement) Texts(ctx context.Context, textOnly bool) ([]string, error) {
	snapshot, err := e.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	return snapshot.Texts(textOnly), nil
}
func (e *ChromiumElement) BoxModel(ctx context.Context) (*proto.DOMBoxModel, error) {
	result, err := (proto.DOMGetBoxModel{ObjectID: e.element.Object.ObjectID}).Call(e.tab.page.Context(ctx))
	if err != nil {
		return nil, err
	}
	return result.Model, nil
}
func (b *Chromium) ActivateTab(ctx context.Context, id string) error {
	tab, err := b.GetTab(ctx, id)
	if err != nil {
		return err
	}
	return tab.Activate(ctx)
}
func (b *Chromium) UserDataPath() string { return b.options.UserDataPath }
func (t *ChromiumTab) settings() ChromiumOptions {
	if t.config != nil {
		return *t.config
	}
	return t.browser.options
}

// Runtime setters are serialized with operations on the same tab. Other tabs
// retain their independent settings; operation contexts may impose shorter limits.
func (t *ChromiumTab) SetTimeouts(base, load, script time.Duration) error {
	if base <= 0 || load <= 0 || script <= 0 {
		return fmt.Errorf("timeouts must be positive")
	}
	settings := t.settings()
	settings.Timeout = base
	settings.PageLoadTimeout = load
	settings.ScriptTimeout = script
	t.config = &settings
	return nil
}
func (t *ChromiumTab) SetLoadMode(mode string) error {
	if mode != "normal" && mode != "eager" && mode != "none" {
		return fmt.Errorf("invalid load mode %q", mode)
	}
	settings := t.settings()
	settings.LoadMode = mode
	t.config = &settings
	return nil
}
func (t *ChromiumTab) SetRetry(times int, interval time.Duration) error {
	if times < 0 || interval < 0 {
		return fmt.Errorf("invalid retry settings")
	}
	settings := t.settings()
	settings.RetryTimes = times
	settings.RetryInterval = interval
	t.config = &settings
	return nil
}
