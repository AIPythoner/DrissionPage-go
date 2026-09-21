package drissionpage

import (
	"context"
	"encoding/json"
)

// Resolve reacquires the frame session from its owner element. This follows
// same-process to OOPIF transitions while the owning iframe element survives.
// If the owner itself was removed, callers must find the replacement iframe.
func (f *ChromiumFrame) Resolve(ctx context.Context) (*ChromiumTab, error) {
	fresh, err := f.element.Frame(ctx)
	if err != nil {
		return nil, err
	}
	return fresh.ChromiumTab, nil
}
func (f *ChromiumFrame) Ele(ctx context.Context, locator any, index ...int) (*ChromiumElement, error) {
	t, e := f.Resolve(ctx)
	if e != nil {
		return nil, e
	}
	return t.Ele(ctx, locator, index...)
}
func (f *ChromiumFrame) Eles(ctx context.Context, locator any) ([]*ChromiumElement, error) {
	t, e := f.Resolve(ctx)
	if e != nil {
		return nil, e
	}
	return t.Eles(ctx, locator)
}
func (f *ChromiumFrame) RunJS(ctx context.Context, function string, args ...any) (json.RawMessage, error) {
	t, e := f.Resolve(ctx)
	if e != nil {
		return nil, e
	}
	return t.RunJS(ctx, function, args...)
}
func (f *ChromiumFrame) RunCDP(ctx context.Context, method string, params any) (json.RawMessage, error) {
	t, e := f.Resolve(ctx)
	if e != nil {
		return nil, e
	}
	return t.RunCDP(ctx, method, params)
}
func (f *ChromiumFrame) HTML(ctx context.Context) (string, error) {
	t, e := f.Resolve(ctx)
	if e != nil {
		return "", e
	}
	return t.HTML(ctx)
}
func (f *ChromiumFrame) Title(ctx context.Context) (string, error) {
	t, e := f.Resolve(ctx)
	if e != nil {
		return "", e
	}
	return t.Title(ctx)
}
func (f *ChromiumFrame) URL(ctx context.Context) (string, error) {
	t, e := f.Resolve(ctx)
	if e != nil {
		return "", e
	}
	return t.URL(ctx)
}
func (f *ChromiumFrame) WaitJS(ctx context.Context, function string, args ...any) error {
	t, e := f.Resolve(ctx)
	if e != nil {
		return e
	}
	return t.WaitJS(ctx, function, args...)
}
func (f *ChromiumFrame) WaitLoad(ctx context.Context) error {
	t, e := f.Resolve(ctx)
	if e != nil {
		return e
	}
	return t.WaitLoad(ctx)
}
func (f *ChromiumFrame) GetFrame(ctx context.Context, locator any, index ...int) (*ChromiumFrame, error) {
	t, e := f.Resolve(ctx)
	if e != nil {
		return nil, e
	}
	return t.GetFrame(ctx, locator, index...)
}
func (f *ChromiumFrame) Frames(ctx context.Context) ([]*ChromiumFrame, error) {
	t, e := f.Resolve(ctx)
	if e != nil {
		return nil, e
	}
	return t.Frames(ctx)
}
func (f *ChromiumFrame) Snapshot(ctx context.Context) (*SessionElement, error) {
	t, e := f.Resolve(ctx)
	if e != nil {
		return nil, e
	}
	return t.Snapshot(ctx)
}
func (f *ChromiumFrame) SEle(ctx context.Context, locator any, index ...int) (*SessionElement, error) {
	t, e := f.Resolve(ctx)
	if e != nil {
		return nil, e
	}
	return t.SEle(ctx, locator, index...)
}
func (f *ChromiumFrame) Rect(ctx context.Context) (*PageGeometry, error) {
	t, e := f.Resolve(ctx)
	if e != nil {
		return nil, e
	}
	return t.Rect(ctx)
}
func (f *ChromiumFrame) Scroll(ctx context.Context, x, y float64) error {
	t, e := f.Resolve(ctx)
	if e != nil {
		return e
	}
	return t.Scroll(ctx, x, y)
}
func (f *ChromiumFrame) ScrollTo(ctx context.Context, x, y float64) error {
	t, e := f.Resolve(ctx)
	if e != nil {
		return e
	}
	return t.ScrollTo(ctx, x, y)
}
func (f *ChromiumFrame) ScrollToBottom(ctx context.Context) error {
	t, e := f.Resolve(ctx)
	if e != nil {
		return e
	}
	return t.ScrollToBottom(ctx)
}
