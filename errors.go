package drissionpage

import (
	"errors"
	"fmt"
)

var (
	ErrElementNotFound = errors.New("element not found")
	ErrInvalidLocator  = errors.New("invalid locator")
	ErrInvalidIndex    = errors.New("index must be nonzero (one-based)")
	ErrClosed          = errors.New("closed")
	ErrNoResponse      = errors.New("no response available")
)

type HTTPError struct {
	StatusCode int
	URL        string
}

func (e *HTTPError) Error() string { return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.URL) }

func selectIndex[T any](items []T, index int) (T, error) {
	var zero T
	if index == 0 {
		return zero, ErrInvalidIndex
	}
	if index < 0 {
		index = len(items) + index + 1
	}
	if index < 1 || index > len(items) {
		return zero, ErrElementNotFound
	}
	return items[index-1], nil
}
