package drissionpage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type SelectOption struct {
	Index              int
	Value, Text        string
	Selected, Disabled bool
}

func (e *ChromiumElement) Options(ctx context.Context) ([]SelectOption, error) {
	v, err := e.RunJS(ctx, `function(){if(this.tagName!=='SELECT')throw new Error('element is not SELECT');return Array.from(this.options,(o,i)=>({Index:i+1,Value:o.value,Text:o.text,Selected:o.selected,Disabled:o.disabled}))}`)
	if err != nil {
		return nil, err
	}
	var out []SelectOption
	err = json.Unmarshal(v, &out)
	return out, err
}
func (e *ChromiumElement) SelectByIndex(ctx context.Context, indices ...int) error {
	for _, index := range indices {
		if index == 0 {
			return ErrInvalidIndex
		}
	}
	return e.selectMatching(ctx, "index", indices, true)
}

// selectMatching preserves other selections for multiple-select elements.
func (e *ChromiumElement) selectMatching(ctx context.Context, kind string, values any, selected bool) error {
	ctx, cancel := context.WithTimeout(ctx, e.tab.settings().Timeout)
	defer cancel()
	return WaitUntil(ctx, 25*time.Millisecond, func() (bool, error) {
		data, err := e.RunJS(ctx, `function(kind,values,selected){
 if(this.tagName!=='SELECT')throw Error('element is not SELECT');
 const options=Array.from(this.options);let matches=[];
 if(kind==='index'){for(const value of values){const option=options[value>0?value-1:options.length+value];if(!option)return false;matches.push(option)}}
 else {for(const value of new Set(values)){const found=options.filter(o=>kind==='value'?o.value===value:kind==='text'?o.text===value:o.matches(value));if(!found.length)return false;matches.push(...found)}}
 matches=Array.from(new Set(matches));if(!this.multiple)matches=matches.slice(0,1);
 for(const option of matches){option.selected=selected;this.dispatchEvent(new CustomEvent('change',{bubbles:true}))};return true
 }`, kind, values, selected)
		if err != nil {
			return false, err
		}
		var ok bool
		err = json.Unmarshal(data, &ok)
		return ok, err
	})
}
func (e *ChromiumElement) CancelByIndex(ctx context.Context, indices ...int) error {
	for _, index := range indices {
		if index == 0 {
			return ErrInvalidIndex
		}
	}
	return e.selectMatching(ctx, "index", indices, false)
}
func (e *ChromiumElement) CancelByText(ctx context.Context, texts ...string) error {
	return e.selectMatching(ctx, "text", texts, false)
}
func (e *ChromiumElement) IsMultiple(ctx context.Context) (bool, error) {
	data, err := e.RunJS(ctx, `function(){if(this.tagName!=='SELECT')throw Error('element is not SELECT');return this.multiple}`)
	if err != nil {
		return false, err
	}
	var value bool
	err = json.Unmarshal(data, &value)
	return value, err
}
func (e *ChromiumElement) SelectAll(ctx context.Context) error { return e.selectMode(ctx, "all") }
func (e *ChromiumElement) ClearSelection(ctx context.Context) error {
	return e.selectMode(ctx, "clear")
}
func (e *ChromiumElement) InvertSelection(ctx context.Context) error {
	return e.selectMode(ctx, "invert")
}
func (e *ChromiumElement) selectMode(ctx context.Context, mode string) error {
	_, err := e.RunJS(ctx, `function(mode){if(this.tagName!=='SELECT')throw new Error('element is not SELECT');if(!this.multiple)throw new Error('select is not multiple');for(const o of this.options)o.selected=mode==='all'||(mode==='invert'&&!o.selected);if(mode==='clear')this.selectedIndex=-1;this.dispatchEvent(new Event('input',{bubbles:true}));this.dispatchEvent(new Event('change',{bubbles:true}))}`, mode)
	return err
}
func (e *ChromiumElement) CancelSelection(ctx context.Context, values ...string) error {
	return e.selectMatching(ctx, "value", values, false)
}
func (e *ChromiumElement) SelectedOptions(ctx context.Context) ([]SelectOption, error) {
	options, err := e.Options(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SelectOption, 0)
	for _, o := range options {
		if o.Selected {
			out = append(out, o)
		}
	}
	return out, nil
}
func (e *ChromiumElement) SelectedOption(ctx context.Context) (*SelectOption, error) {
	options, err := e.SelectedOptions(ctx)
	if err != nil {
		return nil, err
	}
	if len(options) == 0 {
		return nil, fmt.Errorf("%w: no selected option", ErrElementNotFound)
	}
	return &options[0], nil
}
