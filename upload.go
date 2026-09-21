package drissionpage

import (
	"context"
	"fmt"
	"github.com/go-rod/rod/lib/proto"
	"os"
	"path/filepath"
	"time"
)

func uploadPaths(paths []string) ([]string, error) {
	out := make([]string, len(paths))
	for i, path := range paths {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(absolute)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("upload requires regular file: %s", path)
		}
		out[i] = absolute
	}
	return out, nil
}

// ClickToUpload intercepts the browser chooser triggered by clicking this
// element. Serialize chooser operations within the same tab.
func (e *ChromiumElement) ClickToUpload(ctx context.Context, paths ...string) error {
	files, err := uploadPaths(paths)
	if err != nil {
		return err
	}
	life, cancel := context.WithTimeout(ctx, e.tab.browser.options.Timeout)
	defer cancel()
	page := e.tab.page.Context(life)
	wait, err := page.HandleFileDialog()
	if err != nil {
		return err
	}
	defer func() {
		cleanup, c := context.WithTimeout(context.Background(), time.Second)
		defer c()
		_ = (proto.PageSetInterceptFileChooserDialog{Enabled: false}).Call(e.tab.page.Context(cleanup))
	}()
	if err = e.Click(life); err != nil {
		return err
	}
	return wait(files)
}

// DropFiles sends native CDP drag events with local file paths to this element.
func (e *ChromiumElement) DropFiles(ctx context.Context, paths ...string) error {
	files, err := uploadPaths(paths)
	if err != nil {
		return err
	}
	if err = e.ScrollIntoView(ctx); err != nil {
		return err
	}
	rect, err := e.Rect(ctx)
	if err != nil {
		return err
	}
	center := rect.Midpoint()
	data := &proto.InputDragData{Items: []*proto.InputDragDataItem{}, Files: files, DragOperationsMask: 1}
	for _, kind := range []proto.InputDispatchDragEventType{"dragEnter", "dragOver", "drop"} {
		if err = (proto.InputDispatchDragEvent{Type: kind, X: center.X, Y: center.Y, Data: data}).Call(e.tab.page.Context(ctx)); err != nil {
			return err
		}
	}
	return nil
}
