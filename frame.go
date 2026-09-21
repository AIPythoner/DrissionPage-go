package drissionpage

import (
	"context"
	"fmt"
	"github.com/go-rod/rod/lib/proto"
)

func (f *ChromiumFrame) FrameElement() *ChromiumElement { return f.element }
func (f *ChromiumFrame) Owner() *ChromiumTab            { return f.element.tab }
func (f *ChromiumFrame) Attr(ctx context.Context, name string) (string, bool, error) {
	return f.element.Attr(ctx, name)
}
func (f *ChromiumFrame) Attrs(ctx context.Context) (map[string]string, error) {
	return f.element.Attrs(ctx)
}
func (f *ChromiumFrame) Tag(ctx context.Context) (string, error) { return f.element.Tag(ctx) }
func (f *ChromiumFrame) Parent(ctx context.Context) (*ChromiumElement, error) {
	return f.element.Parent(ctx)
}
func (f *ChromiumFrame) Prev(ctx context.Context) (*ChromiumElement, error) {
	return f.element.Prev(ctx)
}
func (f *ChromiumFrame) Next(ctx context.Context) (*ChromiumElement, error) {
	return f.element.Next(ctx)
}
func (f *ChromiumFrame) FrameRect(ctx context.Context) (*Rect, error) { return f.element.Rect(ctx) }
func (f *ChromiumFrame) RemoveAttr(ctx context.Context, name string) error {
	return f.element.RemoveAttr(ctx, name)
}
func (f *ChromiumFrame) SetAttr(ctx context.Context, name, value string) error {
	return f.element.SetAttr(ctx, name, value)
}
func (f *ChromiumFrame) Style(ctx context.Context, name string) (string, error) {
	return f.element.Style(ctx, name)
}
func (f *ChromiumFrame) Path(ctx context.Context) (string, error)    { return f.element.Path(ctx) }
func (f *ChromiumFrame) CSSPath(ctx context.Context) (string, error) { return f.element.CSSPath(ctx) }
func (f *ChromiumFrame) Close(ctx context.Context) error             { return f.element.Remove(ctx) }
func (f *ChromiumFrame) Screenshot(ctx context.Context, path string) ([]byte, error) {
	return f.element.Screenshot(ctx, path)
}
func (f *ChromiumFrame) Get(ctx context.Context, target string) error {
	ctx, cancel := context.WithTimeout(ctx, f.settings().PageLoadTimeout)
	defer cancel()
	current, err := f.Resolve(ctx)
	if err != nil {
		return err
	}
	result, err := (proto.PageNavigate{URL: target, FrameID: current.page.FrameID}).Call(current.page.Context(ctx))
	if err != nil {
		return err
	}
	if result.ErrorText != "" {
		return fmt.Errorf("frame navigation: %s", result.ErrorText)
	}
	fresh, err := f.element.Frame(ctx)
	if err != nil {
		return err
	}
	fresh.config = f.config
	f.ChromiumTab = fresh.ChromiumTab
	if f.settings().LoadMode == "none" {
		return nil
	}
	return f.WaitLoad(ctx)
}
func (f *ChromiumFrame) Refresh(ctx context.Context) error {
	url, err := f.URL(ctx)
	if err != nil {
		return err
	}
	return f.Get(ctx, url)
}
