package drissionpage

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/AIPythoner/DrissionPage-go/internal/rod"
	"github.com/go-rod/rod/lib/proto"
	"os"
	"regexp"
	"strings"
)

var jsFunctionStart = regexp.MustCompile(`^(?:async\s*)?(?:function\b|\([^)]*\)\s*=>|[\pL_$][\pL\pN_$]*\s*=>)`)

func prepareScript(script string, args []any) (string, []any, error) {
	if info, err := os.Stat(script); err == nil && info.Mode().IsRegular() {
		data, err := os.ReadFile(script)
		if err != nil {
			return "", nil, err
		}
		script = string(data)
	}
	script = strings.TrimSpace(script)
	if !jsFunctionStart.MatchString(script) {
		script = "function(){\n" + script + "\n}"
	}
	converted := append([]any(nil), args...)
	for i, arg := range converted {
		switch value := arg.(type) {
		case *ChromiumElement:
			if value == nil {
				return "", nil, fmt.Errorf("nil element argument")
			}
			converted[i] = value.element.Object
		case *JSHandle:
			if value == nil {
				return "", nil, fmt.Errorf("nil JS handle")
			}
			converted[i] = value.Object
		}
	}
	return script, converted, nil
}

// JSHandle retains a JavaScript object, DOM node, array, or unserializable value.
// Release it when finished. Navigation invalidates handles from that document.
type JSHandle struct {
	Object *proto.RuntimeRemoteObject
	tab    *ChromiumTab
}

func (t *ChromiumTab) EvalHandle(ctx context.Context, script string, args ...any) (*JSHandle, error) {
	function, args, err := prepareScript(script, args)
	if err != nil {
		return nil, err
	}
	life, cancel := context.WithTimeout(ctx, t.settings().ScriptTimeout)
	defer cancel()
	object, err := t.page.Context(life).Evaluate(rod.Eval(function, args...).ByObject())
	if err != nil {
		return nil, err
	}
	return &JSHandle{Object: object, tab: t}, nil
}
func (e *ChromiumElement) EvalHandle(ctx context.Context, script string, args ...any) (*JSHandle, error) {
	function, args, err := prepareScript(script, args)
	if err != nil {
		return nil, err
	}
	life, cancel := context.WithTimeout(ctx, e.tab.settings().ScriptTimeout)
	defer cancel()
	object, err := e.element.Context(life).Evaluate(rod.Eval(function, args...).ByObject())
	if err != nil {
		return nil, err
	}
	return &JSHandle{Object: object, tab: e.tab}, nil
}
func (h *JSHandle) Element(ctx context.Context) (*ChromiumElement, error) {
	element, err := h.tab.page.Context(ctx).ElementFromObject(h.Object)
	if err != nil {
		return nil, err
	}
	return &ChromiumElement{element.Context(h.tab.page.GetContext()), h.tab}, nil
}
func (h *JSHandle) JSON(ctx context.Context) (json.RawMessage, error) {
	return h.tab.RunJS(ctx, `value=>value`, h)
}
func (h *JSHandle) Properties(ctx context.Context) (map[string]*JSHandle, error) {
	result, err := (proto.RuntimeGetProperties{ObjectID: h.Object.ObjectID, OwnProperties: true}).Call(h.tab.page.Context(ctx))
	if err != nil {
		return nil, err
	}
	out := map[string]*JSHandle{}
	for _, property := range result.Result {
		if property.Value != nil {
			out[property.Name] = &JSHandle{Object: property.Value, tab: h.tab}
		}
	}
	return out, nil
}
func (h *JSHandle) Release(ctx context.Context) error {
	if h.Object.ObjectID == "" {
		return nil
	}
	return (proto.RuntimeReleaseObject{ObjectID: h.Object.ObjectID}).Call(h.tab.page.Context(ctx))
}
