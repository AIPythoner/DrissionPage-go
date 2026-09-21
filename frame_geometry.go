package drissionpage

import (
	"context"
	"encoding/json"
	"fmt"
)

// RootRect expresses an element's bounding rectangle in top-level viewport CSS
// pixels. Frame borders and axis-aligned CSS scale are included. Rotated/skewed
// iframe transforms cannot be represented by a Rect and return an error.
func (e *ChromiumElement) RootRect(ctx context.Context) (*Rect, error) {
	rect, err := e.Rect(ctx)
	if err != nil {
		return nil, err
	}
	for owner := e.tab.frameOwner; owner != nil; owner = owner.tab.frameOwner {
		data, err := owner.RunJS(ctx, `function(){const r=this.getBoundingClientRect(),s=getComputedStyle(this),m=new DOMMatrix(s.transform);if(m.b!==0||m.c!==0||!m.is2D)throw Error('rotated or skewed iframe requires quad geometry');return {X:r.x,Y:r.y,Width:r.width,Height:r.height,OffsetWidth:this.offsetWidth,OffsetHeight:this.offsetHeight,BorderX:this.clientLeft,BorderY:this.clientTop}}`)
		if err != nil {
			return nil, err
		}
		var frame struct{ X, Y, Width, Height, OffsetWidth, OffsetHeight, BorderX, BorderY float64 }
		if err = json.Unmarshal(data, &frame); err != nil {
			return nil, err
		}
		if frame.OffsetWidth == 0 || frame.OffsetHeight == 0 {
			return nil, fmt.Errorf("frame has no rendered size")
		}
		sx, sy := frame.Width/frame.OffsetWidth, frame.Height/frame.OffsetHeight
		rect = &Rect{X: frame.X + (frame.BorderX+rect.X)*sx, Y: frame.Y + (frame.BorderY+rect.Y)*sy, Width: rect.Width * sx, Height: rect.Height * sy}
	}
	return rect, nil
}

type FrameGeometry struct {
	OwnerViewport, OwnerPage, RootViewport Rect
	Content                                PageGeometry
}

func (f *ChromiumFrame) Geometry(ctx context.Context) (*FrameGeometry, error) {
	view, err := f.element.Rect(ctx)
	if err != nil {
		return nil, err
	}
	page, err := f.element.PageRect(ctx)
	if err != nil {
		return nil, err
	}
	root, err := f.element.RootRect(ctx)
	if err != nil {
		return nil, err
	}
	content, err := f.Rect(ctx)
	if err != nil {
		return nil, err
	}
	return &FrameGeometry{*view, *page, *root, *content}, nil
}

// ScreenRect returns physical screen pixels on Windows. A visible local browser
// window is required. Headless/remote browsers have no native screen rectangle.
func (e *ChromiumElement) ScreenRect(ctx context.Context) (*Rect, error) {
	rect, err := e.RootRect(ctx)
	if err != nil {
		return nil, err
	}
	root := e.tab
	for root.frameOwner != nil {
		root = root.frameOwner.tab
	}
	windows, err := root.NativeWindows(ctx)
	if err != nil {
		return nil, err
	}
	if len(windows) != 1 {
		return nil, fmt.Errorf("screen coordinates require exactly one matching native window")
	}
	geometry, err := root.Rect(ctx)
	if err != nil {
		return nil, err
	}
	origin, err := nativeContentOrigin(windows[0].Handle, geometry.ViewportWidth, geometry.ViewportHeight, geometry.DevicePixelRatio)
	if err != nil {
		return nil, err
	}
	dpr := geometry.DevicePixelRatio
	return &Rect{X: origin.X + rect.X*dpr, Y: origin.Y + rect.Y*dpr, Width: rect.Width * dpr, Height: rect.Height * dpr}, nil
}

func (t *ChromiumTab) topTab() *ChromiumTab {
	for t.frameOwner != nil {
		t = t.frameOwner.tab
	}
	return t
}
