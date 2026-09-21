package drissionpage

import (
	"context"
	"fmt"
	"github.com/go-rod/rod/lib/proto"
	"math"
	"strings"
	"time"
)

func (t *ChromiumTab) WaitElements(ctx context.Context, locators []any, anyOne bool) error {
	if len(locators) == 0 {
		return fmt.Errorf("at least one locator is required")
	}
	ctx, cancel := context.WithTimeout(ctx, t.settings().Timeout)
	defer cancel()
	return WaitUntil(ctx, 25*time.Millisecond, func() (bool, error) {
		all := true
		for _, locator := range locators {
			elements, err := t.Eles(ctx, locator)
			if err != nil {
				return false, err
			}
			found := len(elements) > 0
			if found && anyOne {
				return true, nil
			}
			all = all && found
		}
		return all, nil
	})
}
func (t *ChromiumTab) WaitDisplayed(ctx context.Context, locator any, displayed bool) error {
	ctx, cancel := context.WithTimeout(ctx, t.settings().Timeout)
	defer cancel()
	return WaitUntil(ctx, 25*time.Millisecond, func() (bool, error) {
		elements, err := t.Eles(ctx, locator)
		if err != nil {
			return false, err
		}
		if len(elements) == 0 {
			return !displayed, nil
		}
		for _, element := range elements {
			state, err := element.States(ctx)
			if err != nil {
				return false, err
			}
			if state.Displayed == displayed {
				return true, nil
			}
		}
		return false, nil
	})
}
func (t *ChromiumTab) WaitURLChange(ctx context.Context, text string, exclude bool) error {
	return t.WaitJS(ctx, `(text,exclude)=>location.href.includes(text)!==exclude`, text, exclude)
}
func (t *ChromiumTab) WaitTitleChange(ctx context.Context, text string, exclude bool) error {
	return t.WaitJS(ctx, `(text,exclude)=>document.title.includes(text)!==exclude`, text, exclude)
}
func (e *ChromiumElement) WaitDisabledOrDeleted(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, e.tab.settings().Timeout)
	defer cancel()
	return WaitUntil(ctx, 25*time.Millisecond, func() (bool, error) {
		state, err := e.States(ctx)
		return err == nil && (!state.Alive || !state.Enabled), err
	})
}

// WaitStopMoving requires an unchanged rectangle for the entire quiet interval.
func (e *ChromiumElement) WaitStopMoving(ctx context.Context, quiet time.Duration, tolerance float64) error {
	if quiet <= 0 || tolerance < 0 {
		return fmt.Errorf("invalid movement wait parameters")
	}
	ctx, cancel := context.WithTimeout(ctx, e.tab.settings().Timeout)
	defer cancel()
	var previous *Rect
	since := time.Now()
	return WaitUntil(ctx, 25*time.Millisecond, func() (bool, error) {
		rect, err := e.Rect(ctx)
		if err != nil {
			return false, err
		}
		if previous == nil || math.Abs(rect.X-previous.X) > tolerance || math.Abs(rect.Y-previous.Y) > tolerance || math.Abs(rect.Width-previous.Width) > tolerance || math.Abs(rect.Height-previous.Height) > tolerance {
			previous = rect
			since = time.Now()
		}
		return time.Since(since) >= quiet, nil
	})
}
func (e *ChromiumElement) WaitClickable(ctx context.Context, stopMoving bool) error {
	if err := e.WaitState(ctx, "clickable", true); err != nil {
		return err
	}
	if stopMoving {
		return e.WaitStopMoving(ctx, 100*time.Millisecond, 0.5)
	}
	return nil
}

// DuringLoadStart subscribes before action, so even a fast navigation is seen.
func (t *ChromiumTab) DuringLoadStart(ctx context.Context, action func() error) error {
	life, cancel := context.WithTimeout(ctx, t.settings().PageLoadTimeout)
	defer cancel()
	p := t.page.Context(life)
	started := make(chan struct{}, 1)
	wait := p.EachEvent(func(event *proto.PageFrameStartedLoading) {
		if event.FrameID == p.FrameID {
			select {
			case started <- struct{}{}:
			default:
			}
		}
	})
	go wait()
	if action != nil {
		if err := action(); err != nil {
			return err
		}
	}
	select {
	case <-started:
		return nil
	case <-life.Done():
		return life.Err()
	}
}

// DuringAlert subscribes before action and waits for an opened dialog to close.
// Install AutoHandleAlerts or handle the dialog concurrently when using it.
func (t *ChromiumTab) DuringAlert(ctx context.Context, action func() error) error {
	life, cancel := context.WithTimeout(ctx, t.settings().Timeout)
	defer cancel()
	opened := false
	closed := make(chan struct{}, 1)
	wait := t.page.Context(life).EachEvent(func(*proto.PageJavascriptDialogOpening) { opened = true }, func(*proto.PageJavascriptDialogClosed) {
		if opened {
			select {
			case closed <- struct{}{}:
			default:
			}
		}
	})
	go wait()
	if action != nil {
		if err := action(); err != nil {
			return err
		}
	}
	select {
	case <-closed:
		return nil
	case <-life.Done():
		return life.Err()
	}
}
func (e *ChromiumElement) ClickForURLChange(ctx context.Context) error {
	before, err := e.tab.URL(ctx)
	if err != nil {
		return err
	}
	if err = e.Click(ctx); err != nil {
		return err
	}
	return e.tab.WaitJS(ctx, `before=>location.href!==before`, before)
}
func (e *ChromiumElement) ClickForTitleChange(ctx context.Context) error {
	before, err := e.tab.Title(ctx)
	if err != nil {
		return err
	}
	if err = e.Click(ctx); err != nil {
		return err
	}
	return e.tab.WaitJS(ctx, `before=>document.title!==before`, before)
}
func (d *DownloadManager) WaitAll(ctx context.Context, frameID string, cancelOnTimeout bool) error {
	err := WaitUntil(ctx, 25*time.Millisecond, func() (bool, error) {
		for _, mission := range d.Missions() {
			if (frameID == "" || mission.FrameID == frameID) && mission.State == "inProgress" {
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil && cancelOnTimeout {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, mission := range d.Missions() {
			if (frameID == "" || mission.FrameID == frameID) && mission.State == "inProgress" {
				_ = d.Cancel(cleanup, mission.GUID)
			}
		}
	}
	return err
}

// WaitBegin only considers missions absent from before, without consuming Next.
// URLContains can distinguish concurrent downloads initiated by other clients.
func (d *DownloadManager) WaitBegin(ctx context.Context, before []BrowserDownload, frameID, urlContains string) (BrowserDownload, error) {
	seen := map[string]bool{}
	for _, mission := range before {
		seen[mission.GUID] = true
	}
	var found BrowserDownload
	err := WaitUntil(ctx, 25*time.Millisecond, func() (bool, error) {
		var matches []BrowserDownload
		for _, mission := range d.Missions() {
			if !seen[mission.GUID] && (frameID == "" || mission.FrameID == frameID) && strings.Contains(mission.URL, urlContains) {
				matches = append(matches, mission)
			}
		}
		if len(matches) > 1 {
			return false, fmt.Errorf("multiple downloads match; select a GUID from Missions")
		}
		if len(matches) == 1 {
			found = matches[0]
			return true, nil
		}
		return false, nil
	})
	return found, err
}
