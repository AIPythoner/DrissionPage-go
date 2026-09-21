package drissionpage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/AIPythoner/DrissionPage-go/internal/rod"
	"github.com/go-rod/rod/lib/proto"
)

type Point struct{ X, Y float64 }

func (r Rect) Midpoint() Point { return Point{r.X + r.Width/2, r.Y + r.Height/2} }
func (r Rect) Corners() [4]Point {
	return [4]Point{{r.X, r.Y}, {r.X + r.Width, r.Y}, {r.X + r.Width, r.Y + r.Height}, {r.X, r.Y + r.Height}}
}

type PageGeometry struct{ ScrollX, ScrollY, Width, Height, ViewportWidth, ViewportHeight, ScreenX, ScreenY, OuterWidth, OuterHeight, DevicePixelRatio float64 }

func (t *ChromiumTab) Rect(ctx context.Context) (*PageGeometry, error) {
	v, e := t.RunJS(ctx, `()=>({ScrollX:scrollX,ScrollY:scrollY,Width:document.scrollingElement.scrollWidth,Height:document.scrollingElement.scrollHeight,ViewportWidth:innerWidth,ViewportHeight:innerHeight,ScreenX:screenX,ScreenY:screenY,OuterWidth:outerWidth,OuterHeight:outerHeight,DevicePixelRatio:devicePixelRatio})`)
	if e != nil {
		return nil, e
	}
	var r PageGeometry
	e = json.Unmarshal(v, &r)
	return &r, e
}
func (t *ChromiumTab) Window(ctx context.Context) (*proto.BrowserGetWindowForTargetResult, error) {
	return (proto.BrowserGetWindowForTarget{TargetID: t.page.TargetID}).Call(t.browser.browser.Context(ctx))
}
func (e *ChromiumElement) PageRect(ctx context.Context) (*Rect, error) {
	v, err := e.RunJS(ctx, `function(){const r=this.getBoundingClientRect();return {X:r.x+scrollX,Y:r.y+scrollY,Width:r.width,Height:r.height}}`)
	if err != nil {
		return nil, err
	}
	var r Rect
	err = json.Unmarshal(v, &r)
	return &r, err
}
func (e *ChromiumElement) Over(ctx context.Context) (*ChromiumElement, error) {
	p, c := e.tab.operation(ctx)
	defer c()
	el, err := p.ElementByJS(rod.Eval(`(el)=>{const r=el.getBoundingClientRect();return document.elementFromPoint(r.x+r.width/2,r.y+r.height/2)}`, e.element.Object))
	if err != nil {
		return nil, err
	}
	return &ChromiumElement{el.Context(e.tab.page.GetContext()), e.tab}, nil
}

// Neighbor locates an element in a cardinal direction, ordered by distance.
func (e *ChromiumElement) Neighbor(ctx context.Context, direction string, locator any, index int) (*ChromiumElement, error) {
	if direction != "east" && direction != "west" && direction != "north" && direction != "south" {
		return nil, fmt.Errorf("invalid direction %q", direction)
	}
	if index <= 0 {
		return nil, ErrInvalidIndex
	}
	loc, err := ParseLocator(locator)
	if err != nil {
		return nil, err
	}
	if loc.Kind == "ax" {
		return nil, fmt.Errorf("spatial lookup requires CSS, XPath or text")
	}
	p, c := e.tab.operation(ctx)
	defer c()
	el, err := p.ElementByJS(rod.Eval(`(origin,kind,query,direction,index)=>{let candidates=[];if(kind==='css')candidates=Array.from(document.querySelectorAll(query));else if(kind==='xpath'){const r=document.evaluate(query,document,null,XPathResult.ORDERED_NODE_SNAPSHOT_TYPE,null);for(let i=0;i<r.snapshotLength;i++)candidates.push(r.snapshotItem(i))}else{candidates=Array.from(document.querySelectorAll('*')).filter(e=>e.textContent.includes(query))};const r=origin.getBoundingClientRect(),x=r.x+r.width/2,y=r.y+r.height/2;return candidates.filter(e=>e!==origin&&e.nodeType===1&&!e.contains(origin)).map(e=>({e,r:e.getBoundingClientRect()})).filter(o=>o.r.width&&o.r.height&&({east:o.r.left>=r.right,west:o.r.right<=r.left,north:o.r.bottom<=r.top,south:o.r.top>=r.bottom})[direction]).sort((a,b)=>Math.hypot(a.r.x+a.r.width/2-x,a.r.y+a.r.height/2-y)-Math.hypot(b.r.x+b.r.width/2-x,b.r.y+b.r.height/2-y))[index-1]?.e??null}`, e.element.Object, loc.Kind, loc.Value, direction, index))
	if err != nil {
		return nil, err
	}
	return &ChromiumElement{el.Context(e.tab.page.GetContext()), e.tab}, nil
}
func (e *ChromiumElement) ClickAt(ctx context.Context, x, y float64, button string, count int) error {
	if err := e.ScrollIntoView(ctx); err != nil {
		return err
	}
	r, err := e.RootRect(ctx)
	if err != nil {
		return err
	}
	return e.tab.topTab().Actions(ctx).MoveTo(r.X+x, r.Y+y).Click(button, count).Do()
}
