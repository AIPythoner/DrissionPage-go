package drissionpage

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-rod/rod/lib/proto"
)

func (t *ChromiumTab) ElementAt(ctx context.Context, x, y float64) (*ChromiumElement, error) {
	root := t
	node, err := (proto.DOMGetNodeForLocation{X: int(x), Y: int(y), IncludeUserAgentShadowDOM: true}).Call(root.page.Context(ctx))
	if err != nil {
		return nil, err
	}
	owner := root
	if node.FrameID != "" && node.FrameID != root.page.FrameID {
		owner, err = root.findFrameTab(ctx, node.FrameID)
		if err != nil {
			return nil, err
		}
	}
	page := owner.page
	element, err := page.Context(ctx).ElementFromNode(&proto.DOMNode{BackendNodeID: node.BackendNodeID})
	if err != nil {
		return nil, err
	}
	candidate := &ChromiumElement{element.Context(root.page.GetContext()), owner}
	tag, err := candidate.Tag(ctx)
	if err != nil {
		return nil, err
	}
	if tag == "iframe" || tag == "frame" {
		frame, err := candidate.Frame(ctx)
		if err != nil {
			return nil, err
		}
		if frame.page.SessionID != page.SessionID {
			bounds, err := candidate.Rect(ctx)
			if err != nil {
				return nil, err
			}
			data, err := candidate.RunJS(ctx, `function(){return {X:this.clientLeft,Y:this.clientTop}}`)
			if err != nil {
				return nil, err
			}
			var border Point
			if err = json.Unmarshal(data, &border); err != nil {
				return nil, err
			}
			return frame.ChromiumTab.ElementAt(ctx, x-bounds.X-border.X, y-bounds.Y-border.Y)
		}
	}
	return candidate, nil
}
func (e *ChromiumElement) Offset(ctx context.Context, x, y float64) (*ChromiumElement, error) {
	rect, err := e.RootRect(ctx)
	if err != nil {
		return nil, err
	}
	return e.tab.topTab().ElementAt(ctx, rect.X+x, rect.Y+y)
}

// Direction scans the cardinal ray in eight-pixel steps, like Python east/west/
// north/south. A numeric locatorOrPixels performs a single offset hit test.
// Neighbor remains the separate distance-sorted rectangular search API.
func (e *ChromiumElement) Direction(ctx context.Context, direction string, locatorOrPixels any, index int) (*ChromiumElement, error) {
	if index <= 0 {
		return nil, ErrInvalidIndex
	}
	rect, err := e.RootRect(ctx)
	if err != nil {
		return nil, err
	}
	point := rect.Midpoint()
	dx, dy := 0.0, 0.0
	switch direction {
	case "east":
		point.X = rect.X + rect.Width
		dx = 1
	case "west":
		point.X = rect.X
		dx = -1
	case "north":
		point.Y = rect.Y
		dy = -1
	case "south":
		point.Y = rect.Y + rect.Height
		dy = 1
	default:
		return nil, fmt.Errorf("invalid direction %q", direction)
	}
	if pixels, ok := locatorOrPixels.(int); ok {
		return e.tab.topTab().ElementAt(ctx, point.X+dx*float64(pixels), point.Y+dy*float64(pixels))
	}
	var allowed map[proto.DOMBackendNodeID]bool
	if locatorOrPixels != nil {
		elements, err := e.tab.topTab().Eles(ctx, locatorOrPixels)
		if err != nil {
			return nil, err
		}
		allowed = map[proto.DOMBackendNodeID]bool{}
		for _, element := range elements {
			node, err := element.element.Context(ctx).Describe(0, false)
			if err != nil {
				return nil, err
			}
			allowed[node.BackendNodeID] = true
		}
	}
	geometry, err := e.tab.topTab().Rect(ctx)
	if err != nil {
		return nil, err
	}
	var previous proto.DOMBackendNodeID
	count := 0
	for point.X >= 0 && point.Y >= 0 && point.X < geometry.ViewportWidth && point.Y < geometry.ViewportHeight {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		point.X += dx * 8
		point.Y += dy * 8
		candidate, err := e.tab.topTab().ElementAt(ctx, point.X, point.Y)
		if err != nil {
			continue
		}
		node, err := candidate.element.Context(ctx).Describe(0, false)
		if err != nil {
			continue
		}
		if node.BackendNodeID == previous {
			continue
		}
		previous = node.BackendNodeID
		if allowed == nil || allowed[node.BackendNodeID] {
			count++
			if count == index {
				return candidate, nil
			}
		}
	}
	return nil, ErrElementNotFound
}

func (t *ChromiumTab) findFrameTab(ctx context.Context, id proto.PageFrameID) (*ChromiumTab, error) {
	frames, err := t.Frames(ctx)
	if err != nil {
		return nil, err
	}
	for _, frame := range frames {
		if frame.page.FrameID == id {
			return frame.ChromiumTab, nil
		}
		child, err := frame.ChromiumTab.findFrameTab(ctx, id)
		if err == nil {
			return child, nil
		}
	}
	return nil, fmt.Errorf("frame %s not found in DOM tree", id)
}
