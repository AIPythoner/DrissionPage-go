package drissionpage

import (
	"context"
	"encoding/json"
	"fmt"
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
	for _, i := range indices {
		if i == 0 {
			return ErrInvalidIndex
		}
	}
	_, err := e.RunJS(ctx, `function(indices){if(this.tagName!=='SELECT')throw new Error('element is not SELECT');const selected=indices.map(i=>i>0?i-1:this.options.length+i);if(selected.some(i=>i<0||i>=this.options.length))throw new Error('option index out of range');if(!this.multiple&&selected.length>1)throw new Error('select is not multiple');for(let i=0;i<this.options.length;i++)this.options[i].selected=selected.includes(i);this.dispatchEvent(new Event('input',{bubbles:true}));this.dispatchEvent(new Event('change',{bubbles:true}))}`, indices)
	return err
}
func (e *ChromiumElement) SelectAll(ctx context.Context) error { return e.selectMode(ctx, "all") }
func (e *ChromiumElement) ClearSelection(ctx context.Context) error {
	return e.selectMode(ctx, "clear")
}
func (e *ChromiumElement) InvertSelection(ctx context.Context) error {
	return e.selectMode(ctx, "invert")
}
func (e *ChromiumElement) selectMode(ctx context.Context, mode string) error {
	_, err := e.RunJS(ctx, `function(mode){if(this.tagName!=='SELECT')throw new Error('element is not SELECT');if(mode!=='clear'&&!this.multiple)throw new Error('select is not multiple');for(const o of this.options)o.selected=mode==='all'||(mode==='invert'&&!o.selected);if(mode==='clear')this.selectedIndex=-1;this.dispatchEvent(new Event('input',{bubbles:true}));this.dispatchEvent(new Event('change',{bubbles:true}))}`, mode)
	return err
}
func (e *ChromiumElement) CancelSelection(ctx context.Context, values ...string) error {
	_, err := e.RunJS(ctx, `function(values){if(this.tagName!=='SELECT')throw new Error('element is not SELECT');for(const o of this.options)if(values.includes(o.value))o.selected=false;this.dispatchEvent(new Event('input',{bubbles:true}));this.dispatchEvent(new Event('change',{bubbles:true}))}`, values)
	return err
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
