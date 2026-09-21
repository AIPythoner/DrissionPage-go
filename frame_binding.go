package drissionpage

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-rod/rod/lib/cdp"
	"time"
)

// Resolve reacquires the frame session from its owner element. This follows
// same-process to OOPIF transitions while the owning iframe element survives.
// If the owner itself was removed, callers must find the replacement iframe.
func (f *ChromiumFrame) Resolve(ctx context.Context) (*ChromiumTab, error) {
	fresh, err := f.element.Frame(ctx)
	if err != nil {
		return nil, err
	}
	if f.config != nil {
		fresh.config = f.config
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

// URL retries read-only access when a renderer swap invalidates the session
// between Resolve and evaluation. Arbitrary RunJS is never blindly replayed.
func (f *ChromiumFrame) URL(ctx context.Context) (string, error) {
	life, cancel := context.WithTimeout(ctx, f.settings().Timeout)
	defer cancel()
	for {
		attempt, stop := context.WithTimeout(life, 300*time.Millisecond)
		tab, err := f.Resolve(attempt)
		var value string
		if err == nil {
			value, err = tab.URL(attempt)
		}
		stop()
		if err == nil {
			return value, nil
		}
		if life.Err() != nil {
			return "", life.Err()
		}
		if !isFrameTransitionError(err) {
			return "", err
		}
		if err = waitDuration(life, 25*time.Millisecond); err != nil {
			return "", err
		}
	}
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

func isFrameTransitionError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, cdp.ErrCtxDestroyed) || errors.Is(err, cdp.ErrCtxNotFound) || errors.Is(err, cdp.ErrObjNotFound) || errors.Is(err, cdp.ErrSessionNotFound) || errors.Is(err, cdp.ErrNotAttachedToActivePage) {
		return true
	}
	var protocol *cdp.Error
	return errors.As(err, &protocol) && protocol.Code == -32000 && (protocol.Message == "Node with given id does not belong to the document" || protocol.Message == "Cannot find default execution context" || protocol.Message == "No node with given id found")
}
