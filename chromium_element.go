package drissionpage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AIPythoner/DrissionPage-go/internal/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	"github.com/ysmood/gson"
)

func toJSON(v any) gson.JSON { return gson.New(v) }

type ChromiumElement struct {
	element *rod.Element
	tab     *ChromiumTab
}

func (e *ChromiumElement) Raw() *rod.Element { return e.element }
func (e *ChromiumElement) operation(ctx context.Context) (*rod.Element, context.CancelFunc) {
	c, cancel := context.WithTimeout(ctx, e.tab.settings().Timeout)
	return e.element.Context(c), cancel
}
func (e *ChromiumElement) Text(ctx context.Context) (string, error) {
	el, c := e.operation(ctx)
	defer c()
	return el.Text()
}
func (e *ChromiumElement) HTML(ctx context.Context) (string, error) {
	el, c := e.operation(ctx)
	defer c()
	return el.HTML()
}
func (e *ChromiumElement) Attr(ctx context.Context, name string) (string, bool, error) {
	el, c := e.operation(ctx)
	defer c()
	v, err := el.Attribute(name)
	if err != nil {
		return "", false, err
	}
	if v == nil {
		return "", false, nil
	}
	if name == "href" || name == "src" {
		r, err := el.Property(name)
		if err != nil {
			return "", false, err
		}
		return r.Str(), true, nil
	}
	return *v, true, nil
}
func (e *ChromiumElement) Property(ctx context.Context, name string) (json.RawMessage, error) {
	el, c := e.operation(ctx)
	defer c()
	v, err := el.Property(name)
	if err != nil {
		return nil, err
	}
	return v.MarshalJSON()
}
func (e *ChromiumElement) RunJS(ctx context.Context, function string, args ...any) (json.RawMessage, error) {
	function, args, prepErr := prepareScript(function, args)
	if prepErr != nil {
		return nil, prepErr
	}
	el, c := e.operation(ctx)
	defer c()
	v, err := el.Eval(function, args...)
	if err != nil {
		return nil, err
	}
	return v.Value.MarshalJSON()
}
func (e *ChromiumElement) Click(ctx context.Context) error {
	return e.mouseClick(ctx, proto.InputMouseButtonLeft, 1)
}
func (e *ChromiumElement) ClickJS(ctx context.Context) error {
	_, err := e.RunJS(ctx, `function(){this.click()}`)
	return err
}
func (e *ChromiumElement) RightClick(ctx context.Context) error {
	return e.mouseClick(ctx, proto.InputMouseButtonRight, 1)
}
func (e *ChromiumElement) DoubleClick(ctx context.Context) error {
	return e.mouseClick(ctx, proto.InputMouseButtonLeft, 2)
}
func (e *ChromiumElement) Input(ctx context.Context, text string, clear ...bool) error {
	el, c := e.operation(ctx)
	defer c()
	if len(clear) == 0 || clear[0] {
		if err := el.SelectAllText(); err != nil {
			return err
		}
	}
	return el.Input(text)
}
func (e *ChromiumElement) Clear(ctx context.Context) error {
	el, c := e.operation(ctx)
	defer c()
	if err := el.SelectAllText(); err != nil {
		return err
	}
	return el.Type(input.Backspace)
}
func (e *ChromiumElement) Type(ctx context.Context, keys ...input.Key) error {
	el, c := e.operation(ctx)
	defer c()
	return el.Type(keys...)
}
func (e *ChromiumElement) Hover(ctx context.Context) error {
	el, c := e.operation(ctx)
	defer c()
	if err := el.WaitVisible(); err != nil {
		return err
	}
	for {
		if err := (proto.DOMScrollIntoViewIfNeeded{ObjectID: el.Object.ObjectID}).Call(el); err != nil {
			return err
		}
		point, err := el.Interactable()
		if err == nil {
			return e.tab.page.Context(el.GetContext()).Mouse.MoveTo(*point)
		}
		if err = waitDuration(el.GetContext(), 25*time.Millisecond); err != nil {
			return err
		}
	}
}

// Avoid Rod's root requestAnimationFrame wait: background tabs can suspend it
// beyond the operation deadline after window.open activates a different tab.
func (e *ChromiumElement) mouseClick(ctx context.Context, button proto.InputMouseButton, count int) error {
	ctx, cancel := context.WithTimeout(ctx, e.tab.settings().Timeout)
	defer cancel()
	if err := e.element.Context(ctx).WaitEnabled(); err != nil {
		return err
	}
	if err := e.Hover(ctx); err != nil {
		return err
	}
	return e.tab.page.Context(ctx).Mouse.Click(button, count)
}
func (e *ChromiumElement) Focus(ctx context.Context) error {
	el, c := e.operation(ctx)
	defer c()
	return el.Focus()
}
func (e *ChromiumElement) ScrollIntoView(ctx context.Context) error {
	el, c := e.operation(ctx)
	defer c()
	if err := el.WaitVisible(); err != nil {
		return err
	}
	return (proto.DOMScrollIntoViewIfNeeded{ObjectID: el.Object.ObjectID}).Call(el)
}
func (e *ChromiumElement) SetFiles(ctx context.Context, paths ...string) error {
	el, c := e.operation(ctx)
	defer c()
	return el.SetFiles(paths)
}
func (e *ChromiumElement) Select(ctx context.Context, values []string, selected bool) error {
	return e.selectMatching(ctx, "css", values, selected)
}
func (e *ChromiumElement) SelectByText(ctx context.Context, texts ...string) error {
	return e.selectMatching(ctx, "text", texts, true)
}
func (e *ChromiumElement) SelectByValue(ctx context.Context, values ...string) error {
	return e.selectMatching(ctx, "value", values, true)
}
func (e *ChromiumElement) Check(ctx context.Context, checked bool) error {
	_, err := e.RunJS(ctx, `function(checked){if(this.checked!==checked)this.click()}`, checked)
	return err
}
func (e *ChromiumElement) Visible(ctx context.Context) (bool, error) {
	el, c := e.operation(ctx)
	defer c()
	return el.Visible()
}
func (e *ChromiumElement) Enabled(ctx context.Context) (bool, error) {
	v, err := e.RunJS(ctx, `function(){return !this.disabled}`)
	if err != nil {
		return false, err
	}
	var enabled bool
	err = json.Unmarshal(v, &enabled)
	return enabled, err
}
func (e *ChromiumElement) WaitVisible(ctx context.Context) error {
	el, c := e.operation(ctx)
	defer c()
	return el.WaitVisible()
}
func (e *ChromiumElement) WaitHidden(ctx context.Context) error {
	el, c := e.operation(ctx)
	defer c()
	return el.WaitInvisible()
}
func (e *ChromiumElement) WaitStable(ctx context.Context, d time.Duration) error {
	el, c := e.operation(ctx)
	defer c()
	return el.WaitStable(d)
}
func (e *ChromiumElement) Remove(ctx context.Context) error {
	el, c := e.operation(ctx)
	defer c()
	return el.Remove()
}
func (e *ChromiumElement) Frame(ctx context.Context) (*ChromiumFrame, error) {
	ctx, cancel := context.WithTimeout(ctx, e.tab.settings().Timeout)
	defer cancel()
	for {
		frame, err := e.frameOnce(ctx)
		if !errors.Is(err, errFrameNotReady) && !isFrameTransitionError(err) {
			return frame, err
		}
		if err = waitDuration(ctx, 25*time.Millisecond); err != nil {
			return nil, err
		}
	}
}

var errFrameNotReady = errors.New("frame document is transitioning between renderer processes")

func (e *ChromiumElement) frameOnce(ctx context.Context) (*ChromiumFrame, error) {
	el, c := e.operation(ctx)
	defer c()
	// Validate using the caller's deadline, then retain the browser lifetime in
	// Rod's frame-owner element. Rod keeps that element for later frame reloads.
	node, err := el.Describe(1, false)
	if err != nil {
		return nil, err
	}
	targets, err := (proto.TargetGetTargets{}).Call(e.tab.browser.browser.Context(ctx))
	if err != nil {
		return nil, err
	}
	for _, target := range targets.TargetInfos {
		if target.TargetID == proto.TargetTargetID(node.FrameID) && string(target.Type) == "iframe" {
			page, err := e.tab.browser.browser.PageFromTarget(target.TargetID)
			if err != nil {
				return nil, err
			}
			return &ChromiumFrame{ChromiumTab: &ChromiumTab{page: page, browser: e.tab.browser, frameOwner: e, config: e.tab.config}, element: e}, nil
		}
	}
	// A frame can temporarily have neither a local document nor an OOPIF target.
	// Passing it to Rod here would dereference a nil ContentDocument during JS.
	node, err = el.Describe(1, true)
	if err != nil {
		return nil, err
	}
	if node.ContentDocument == nil {
		return nil, errFrameNotReady
	}
	owner, err := e.tab.page.ElementFromObject(e.element.Object)
	if err != nil {
		return nil, err
	}
	p, err := owner.Frame()
	if err != nil {
		return nil, err
	}
	return &ChromiumFrame{ChromiumTab: &ChromiumTab{page: p.Context(e.tab.page.GetContext()), browser: e.tab.browser, frameOwner: e, config: e.tab.config}, element: e}, nil
}
func (e *ChromiumElement) ShadowRoot(ctx context.Context) (*ChromiumElement, error) {
	el, c := e.operation(ctx)
	defer c()
	root, err := el.ShadowRoot()
	if err != nil {
		return nil, err
	}
	return &ChromiumElement{root.Context(e.tab.page.GetContext()), e.tab}, nil
}
func (e *ChromiumElement) Eles(ctx context.Context, value any) ([]*ChromiumElement, error) {
	loc, err := ParseLocator(value)
	if err != nil {
		return nil, err
	}
	el, c := e.operation(ctx)
	defer c()
	var els rod.Elements
	if loc.Kind == "ax" {
		candidates, err := e.tab.Eles(ctx, value)
		if err != nil {
			return nil, err
		}
		out := make([]*ChromiumElement, 0)
		for _, candidate := range candidates {
			contains, err := el.ContainsElement(candidate.element)
			if err != nil {
				return nil, err
			}
			if contains {
				out = append(out, candidate)
			}
		}
		return out, nil
	}
	if loc.Kind == "search" {
		if strings.HasPrefix(loc.Value, "#") || strings.HasPrefix(loc.Value, ".") {
			loc = CSS(loc.Value)
		} else {
			loc = XPath(".//*/text()[contains(.," + xpathLiteral(loc.Value) + ")]/..")
		}
	}
	if loc.Kind == "css" {
		els, err = el.Elements(loc.Value)
	} else {
		q := loc.Value
		if len(q) >= 2 && q[:2] == "//" {
			q = "." + q
		}
		els, err = el.ElementsX(q)
	}
	if err != nil {
		return nil, err
	}
	out := make([]*ChromiumElement, 0, len(els))
	for _, v := range els {
		out = append(out, &ChromiumElement{v.Context(e.tab.page.GetContext()), e.tab})
	}
	return out, nil
}
func (e *ChromiumElement) Ele(ctx context.Context, value any, index ...int) (*ChromiumElement, error) {
	i := 1
	if len(index) > 0 {
		i = index[0]
	}
	if i == 0 {
		return nil, ErrInvalidIndex
	}
	ctx, cancel := context.WithTimeout(ctx, e.tab.settings().Timeout)
	defer cancel()
	for {
		els, err := e.Eles(ctx, value)
		if err != nil {
			return nil, err
		}
		el, err := selectIndex(els, i)
		if err == nil {
			return el, nil
		}
		if err = waitDuration(ctx, 50*time.Millisecond); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrElementNotFound, err)
		}
	}
}
func (e *ChromiumElement) Parent(ctx context.Context) (*ChromiumElement, error) {
	return e.Ele(ctx, XPath(".."))
}
func (e *ChromiumElement) Next(ctx context.Context) (*ChromiumElement, error) {
	return e.Ele(ctx, XPath("following-sibling::*[1]"))
}
func (e *ChromiumElement) Prev(ctx context.Context) (*ChromiumElement, error) {
	return e.Ele(ctx, XPath("preceding-sibling::*[1]"))
}
func (e *ChromiumElement) Children(ctx context.Context) ([]*ChromiumElement, error) {
	return e.Eles(ctx, XPath("./*"))
}
func (e *ChromiumElement) Screenshot(ctx context.Context, path string) ([]byte, error) {
	el, c := e.operation(ctx)
	defer c()
	data, err := el.Screenshot(proto.PageCaptureScreenshotFormatPng, 0)
	if err == nil && path != "" {
		err = os.WriteFile(path, data, 0600)
	}
	return data, err
}

type Rect struct{ X, Y, Width, Height float64 }

func (e *ChromiumElement) Rect(ctx context.Context) (*Rect, error) {
	data, err := e.RunJS(ctx, `function(){const r=this.getBoundingClientRect();return {X:r.x,Y:r.y,Width:r.width,Height:r.height}}`)
	if err != nil {
		return nil, err
	}
	var r Rect
	err = json.Unmarshal(data, &r)
	return &r, err
}
