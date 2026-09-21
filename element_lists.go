package drissionpage

import "context"

// Named slice adapters preserve Go indexing/ranging while offering bulk helpers.
type SessionElements []*SessionElement
type ChromiumElements []*ChromiumElement

func (elements SessionElements) Filter(predicate func(*SessionElement) bool) SessionElements {
	out := SessionElements{}
	for _, element := range elements {
		if predicate(element) {
			out = append(out, element)
		}
	}
	return out
}
func (elements SessionElements) First(predicate func(*SessionElement) bool) (*SessionElement, error) {
	for _, element := range elements {
		if predicate(element) {
			return element, nil
		}
	}
	return nil, ErrElementNotFound
}
func (elements SessionElements) Texts() []string {
	out := make([]string, 0, len(elements))
	for _, element := range elements {
		out = append(out, element.Text())
	}
	return out
}
func (elements SessionElements) Attributes(name string) []string {
	out := make([]string, 0, len(elements))
	for _, element := range elements {
		value, _ := element.Attr(name)
		out = append(out, value)
	}
	return out
}
func (elements ChromiumElements) Filter(ctx context.Context, predicate func(context.Context, *ChromiumElement) (bool, error)) (ChromiumElements, error) {
	out := ChromiumElements{}
	for _, element := range elements {
		ok, err := predicate(ctx, element)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, element)
		}
	}
	return out, nil
}
func (elements ChromiumElements) First(ctx context.Context, predicate func(context.Context, *ChromiumElement) (bool, error)) (*ChromiumElement, error) {
	for _, element := range elements {
		ok, err := predicate(ctx, element)
		if err != nil {
			return nil, err
		}
		if ok {
			return element, nil
		}
	}
	return nil, ErrElementNotFound
}
func (elements ChromiumElements) Texts(ctx context.Context) ([]string, error) {
	out := make([]string, 0, len(elements))
	for _, element := range elements {
		value, err := element.Text(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}
func (elements ChromiumElements) Attributes(ctx context.Context, name string) ([]string, error) {
	out := make([]string, 0, len(elements))
	for _, element := range elements {
		value, _, err := element.Attr(ctx, name)
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}
