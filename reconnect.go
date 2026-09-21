package drissionpage

import (
	"context"
	"fmt"
	"github.com/AIPythoner/DrissionPage-go/internal/rod"
)

// Disconnect closes only the CDP connection. An owned browser remains running
// until Close. Listeners on the disconnected connection terminate.
// Lifecycle changes must be serialized with browser operations.
func (b *Chromium) Disconnect() error {
	if b.closed {
		return ErrClosed
	}
	if b.endpoint == "" {
		return fmt.Errorf("disconnect the root browser, not an isolated context")
	}
	b.connectionCancel()
	return nil
}

// Reconnect establishes a fresh CDP session to the same browser. ctx controls
// its lifetime. Reacquire tabs with GetTab and reinstall listeners afterward;
// old elements and CDP sessions belong to the previous connection.
func (b *Chromium) Reconnect(ctx context.Context) error {
	if b.closed {
		return ErrClosed
	}
	if b.endpoint == "" {
		return fmt.Errorf("reconnect the root browser, not an isolated context")
	}
	life, cancel := context.WithCancel(ctx)
	browser := rod.New().Context(life).ControlURL(b.endpoint).NoDefaultDevice()
	if err := browser.Connect(); err != nil {
		cancel()
		return err
	}
	b.connectionCancel()
	b.browser, b.connectionCancel = browser, cancel
	return nil
}

func (b *Chromium) DebuggerURL() string { return b.endpoint }
