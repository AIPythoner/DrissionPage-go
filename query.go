package drissionpage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AIPythoner/DrissionPage-go/internal/htmlquery"
	"github.com/AIPythoner/DrissionPage-go/internal/rod"
	"github.com/AIPythoner/DrissionPage-go/internal/xpath"
	"github.com/go-rod/rod/lib/proto"
)

func accessibilityElements(p *rod.Page, value string) (rod.Elements, error) {
	role, name := "", ""
	for _, part := range strings.Split(value, "@") {
		i := strings.IndexAny(part, "=:")
		if i < 0 {
			continue
		}
		switch part[:i] {
		case "role":
			role = part[i+1:]
		case "name", "accessibleName":
			name = part[i+1:]
		}
	}
	tree, e := (proto.AccessibilityGetFullAXTree{FrameID: p.FrameID}).Call(p)
	if e != nil {
		return nil, e
	}
	var els rod.Elements
	for _, node := range tree.Nodes {
		if node.Ignored || node.BackendDOMNodeID == 0 {
			continue
		}
		if role != "" && (node.Role == nil || node.Role.Value.Str() != role) {
			continue
		}
		if name != "" && (node.Name == nil || node.Name.Value.Str() != name) {
			continue
		}
		el, e := p.ElementFromNode(&proto.DOMNode{BackendNodeID: node.BackendDOMNodeID})
		if e != nil {
			return nil, e
		}
		els = append(els, el)
	}
	return els, nil
}

// XPathValues evaluates scalar XPath or returns node text/attribute values.
// Use Ele/Eles when DOM element handles are needed.
func (e *SessionElement) XPathValues(expression string) (any, error) {
	compiled, err := xpath.Compile(expression)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidLocator, err)
	}
	result := compiled.Evaluate(htmlquery.CreateXPathNavigator(e.node))
	if iterator, ok := result.(*xpath.NodeIterator); ok {
		values := make([]string, 0)
		for iterator.MoveNext() {
			values = append(values, iterator.Current().Value())
		}
		return values, nil
	}
	return result, nil
}
func (t *ChromiumTab) XPathValues(ctx context.Context, expression string) (json.RawMessage, error) {
	return t.RunJS(ctx, `(expr)=>{const r=document.evaluate(expr,document,null,XPathResult.ANY_TYPE,null);switch(r.resultType){case XPathResult.NUMBER_TYPE:return r.numberValue;case XPathResult.STRING_TYPE:return r.stringValue;case XPathResult.BOOLEAN_TYPE:return r.booleanValue;default:const out=[];let n;while(n=r.iterateNext())out.push(n.nodeValue??n.textContent);return out}}`, expression)
}
func (e *ChromiumElement) XPathValues(ctx context.Context, expression string) (json.RawMessage, error) {
	return e.RunJS(ctx, `function(expr){const r=this.ownerDocument.evaluate(expr,this,null,XPathResult.ANY_TYPE,null);switch(r.resultType){case XPathResult.NUMBER_TYPE:return r.numberValue;case XPathResult.STRING_TYPE:return r.stringValue;case XPathResult.BOOLEAN_TYPE:return r.booleanValue;default:const out=[];let n;while(n=r.iterateNext())out.push(n.nodeValue??n.textContent);return out}}`, expression)
}
func (e *ChromiumElement) Path(ctx context.Context) (string, error) {
	v, err := e.RunJS(ctx, `function(){const parts=[];let n=this;while(n&&n.nodeType===1){let i=1;for(let s=n.previousElementSibling;s;s=s.previousElementSibling)if(s.tagName===n.tagName)i++;parts.unshift(n.tagName.toLowerCase()+'['+i+']');n=n.parentElement}return '/'+parts.join('/')}`)
	if err != nil {
		return "", err
	}
	var s string
	err = json.Unmarshal(v, &s)
	return s, err
}
func (e *ChromiumElement) CSSPath(ctx context.Context) (string, error) {
	v, err := e.RunJS(ctx, `function(){const parts=[];let n=this;while(n&&n.nodeType===1){let i=1;for(let s=n.previousElementSibling;s;s=s.previousElementSibling)if(s.tagName===n.tagName)i++;parts.unshift(n.tagName.toLowerCase()+':nth-of-type('+i+')');n=n.parentElement}return parts.join(' > ')}`)
	if err != nil {
		return "", err
	}
	var s string
	err = json.Unmarshal(v, &s)
	return s, err
}
