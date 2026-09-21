package drissionpage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type FileExistsMode string

const (
	FileRename    FileExistsMode = "rename"
	FileOverwrite FileExistsMode = "overwrite"
	FileSkip      FileExistsMode = "skip"
	FileError     FileExistsMode = "error"
)

// Finalize applies the requested filename to a completed GUID download. The
// filename is a single leaf name inside the manager's configured directory.
func (d *DownloadManager) Finalize(ctx context.Context, guid, filename string, mode FileExistsMode) (string, error) {
	m, e := d.Wait(ctx, guid)
	if e != nil {
		return "", e
	}
	if filename == "" {
		filename = m.SuggestedFilename
	}
	if filename == "" || filename == "." || filename == ".." || strings.ContainsAny(filename, "/\\\x00:") {
		return "", fmt.Errorf("invalid download filename %q", filename)
	}
	target := filepath.Join(d.directory, filename)
	if target == m.Path {
		return target, nil
	}
	switch mode {
	case FileOverwrite:
		if e = os.Rename(m.Path, target); e != nil {
			return "", e
		}
	case FileSkip:
		if _, e = os.Stat(target); e == nil {
			return target, nil
		} else if !os.IsNotExist(e) {
			return "", e
		}
		if e = os.Link(m.Path, target); e != nil {
			return "", e
		}
		if e = os.Remove(m.Path); e != nil {
			return target, e
		}
	case FileRename, FileError, "":
		base, ext := strings.TrimSuffix(filename, filepath.Ext(filename)), filepath.Ext(filename)
		for i := 0; ; i++ {
			if e = ctx.Err(); e != nil {
				return "", e
			}
			if i > 0 {
				target = filepath.Join(d.directory, fmt.Sprintf("%s (%d)%s", base, i, ext))
			}
			e = os.Link(m.Path, target)
			if e == nil {
				break
			}
			if !os.IsExist(e) || mode != FileRename {
				return "", e
			}
		}
		if e = os.Remove(m.Path); e != nil {
			return target, e
		}
	default:
		return "", fmt.Errorf("invalid file exists mode %q", mode)
	}
	d.mu.Lock()
	m.Path = target
	d.missions[guid] = m
	d.mu.Unlock()
	return target, nil
}
func (e *ChromiumElement) ClickToDownload(ctx context.Context, d *DownloadManager) (BrowserDownload, error) {
	// Serialize this helper per initiating frame. Different frames remain independent.
	frame := string(e.tab.page.FrameID)
	d.mu.Lock()
	if d.clickSlots == nil {
		d.clickSlots = map[string]chan struct{}{}
	}
	slot := d.clickSlots[frame]
	if slot == nil {
		slot = make(chan struct{}, 1)
		d.clickSlots[frame] = slot
	}
	d.mu.Unlock()
	select {
	case slot <- struct{}{}:
		defer func() { <-slot }()
	case <-ctx.Done():
		return BrowserDownload{}, ctx.Err()
	}
	known := map[string]bool{}
	for _, m := range d.Missions() {
		known[m.GUID] = true
	}
	if err := e.Click(ctx); err != nil {
		return BrowserDownload{}, err
	}
	for {
		var candidates []BrowserDownload
		for _, m := range d.Missions() {
			if !known[m.GUID] && m.FrameID == frame {
				candidates = append(candidates, m)
			}
		}
		if len(candidates) > 1 {
			return BrowserDownload{}, fmt.Errorf("multiple downloads started in frame %s; select a mission by GUID", frame)
		}
		if len(candidates) == 1 {
			return d.Wait(ctx, candidates[0].GUID)
		}
		if err := waitDuration(ctx, 20*time.Millisecond); err != nil {
			return BrowserDownload{}, err
		}
	}
}
