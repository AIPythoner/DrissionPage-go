package drissionpage

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (o *ChromiumOptions) SetFlag(name string, value any) *ChromiumOptions {
	if o.Flags == nil {
		o.Flags = map[string]any{}
	}
	if disabled, ok := value.(bool); ok && !disabled {
		delete(o.Flags, name)
	} else {
		o.Flags[name] = value
	}
	return o
}
func (o *ChromiumOptions) ClearFlagsInFile() *ChromiumOptions { o.ClearFileFlags = true; return o }
func (o *ChromiumOptions) ClearFlags() *ChromiumOptions       { o.Flags = map[string]any{}; return o }
func (o *ChromiumOptions) ClearArguments() *ChromiumOptions   { o.Arguments = nil; return o }
func (o *ChromiumOptions) ClearPrefs() *ChromiumOptions       { o.Preferences = map[string]any{}; return o }
func (o *ChromiumOptions) RemovePref(name string) *ChromiumOptions {
	delete(o.Preferences, name)
	return o
}
func (o *ChromiumOptions) RemovePrefFromFile(name string) *ChromiumOptions {
	o.DeletePreferences = append(o.DeletePreferences, name)
	return o
}
func (o *ChromiumOptions) RemoveExtensions() *ChromiumOptions { o.Extensions = nil; return o }
func (o *ChromiumOptions) SetUser(name string) *ChromiumOptions {
	return o.Argument("--profile-directory", name)
}
func (o *ChromiumOptions) SetTempPath(path string) *ChromiumOptions { o.TempPath = path; return o }
func (o *ChromiumOptions) SetDownloadPath(path string) *ChromiumOptions {
	o.DownloadPath = path
	return o
}
func (o *ChromiumOptions) CopySystemProfile(path string) *ChromiumOptions {
	o.SystemProfilePath = path
	return o
}
func (o *ChromiumOptions) NewEnv(on bool) *ChromiumOptions       { o.NewEnvironment = on; return o }
func (o *ChromiumOptions) OnlyExisting(on bool) *ChromiumOptions { o.ExistingOnly = on; return o }
func (o *ChromiumOptions) SetCachePath(path string) *ChromiumOptions {
	return o.Argument("--disk-cache-dir", path)
}
func (o *ChromiumOptions) Incognito(on bool) *ChromiumOptions {
	if on {
		return o.Argument("--incognito", "")
	}
	return o.RemoveArgument("--incognito")
}
func (o *ChromiumOptions) IgnoreCertificateErrors(on bool) *ChromiumOptions {
	if on {
		return o.Argument("--ignore-certificate-errors", "")
	}
	return o.RemoveArgument("--ignore-certificate-errors")
}

func profileName(o ChromiumOptions) (string, error) {
	name := "Default"
	for _, arg := range o.Arguments {
		if strings.HasPrefix(arg, "--profile-directory=") {
			name = strings.TrimPrefix(arg, "--profile-directory=")
		}
	}
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\:") {
		return "", fmt.Errorf("invalid profile name %q", name)
	}
	return name, nil
}
func prepareBrowserProfile(o ChromiumOptions) error {
	name, err := profileName(o)
	if err != nil {
		return err
	}
	if o.SystemProfilePath != "" {
		if err = copyProfile(o.SystemProfilePath, o.UserDataPath); err != nil {
			return err
		}
	}
	if o.Preferences != nil || len(o.DeletePreferences) > 0 {
		data, err := browserPreferences(o)
		if err != nil {
			return err
		}
		var prefs map[string]any
		if err = json.Unmarshal(data, &prefs); err != nil {
			return err
		}
		for _, key := range o.DeletePreferences {
			parts := strings.Split(key, ".")
			current := prefs
			for _, part := range parts[:len(parts)-1] {
				next, ok := current[part].(map[string]any)
				if !ok {
					current = nil
					break
				}
				current = next
			}
			if current != nil {
				delete(current, parts[len(parts)-1])
			}
		}
		if err = writeProfileJSON(filepath.Join(o.UserDataPath, name, "Preferences"), prefs); err != nil {
			return err
		}
	}
	if len(o.Flags) > 0 || o.ClearFileFlags {
		path := filepath.Join(o.UserDataPath, "Local State")
		state := map[string]any{}
		if data, e := os.ReadFile(path); e == nil {
			if e = json.Unmarshal(data, &state); e != nil {
				return e
			}
		} else if !os.IsNotExist(e) {
			return e
		}
		browser, ok := state["browser"].(map[string]any)
		if !ok {
			browser = map[string]any{}
			state["browser"] = browser
		}
		flags := map[string]string{}
		if !o.ClearFileFlags {
			if values, ok := browser["enabled_labs_experiments"].([]any); ok {
				for _, raw := range values {
					if value, ok := raw.(string); ok {
						parts := strings.SplitN(value, "@", 2)
						suffix := ""
						if len(parts) == 2 {
							suffix = parts[1]
						}
						flags[parts[0]] = suffix
					}
				}
			}
		}
		for key, value := range o.Flags {
			if value == nil {
				flags[key] = ""
			} else {
				flags[key] = fmt.Sprint(value)
			}
		}
		keys := make([]string, 0, len(flags))
		for key := range flags {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		values := make([]string, 0, len(keys))
		for _, key := range keys {
			if flags[key] != "" {
				values = append(values, key+"@"+flags[key])
			} else {
				values = append(values, key)
			}
		}
		browser["enabled_labs_experiments"] = values
		return writeProfileJSON(path, state)
	}
	return nil
}
func writeProfileJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".profile-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(temp, path)
}
func copyProfile(source, target string) error {
	src, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	dst, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	src, err = filepath.EvalSymlinks(src)
	if err != nil {
		return err
	}
	dst, err = canonicalDestination(dst)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(src, dst)
	if err != nil {
		return err
	}
	if relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("profile destination must be outside source")
	}
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if strings.HasPrefix(entry.Name(), "Singleton") || entry.Name() == "DevToolsActivePort" {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(dst, rel)
		if info, e := os.Lstat(destination); e == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("profile destination contains a symlink: %s", destination)
		}
		if entry.IsDir() {
			return os.MkdirAll(destination, 0700)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		from, err := os.Open(path)
		if err != nil {
			return err
		}
		defer from.Close()
		to, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if os.IsExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		_, err = io.Copy(to, from)
		closeErr := to.Close()
		if err != nil {
			return err
		}
		return closeErr
	})
}

func canonicalDestination(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	parent := filepath.Dir(path)
	if parent == path {
		return "", err
	}
	resolved, err = canonicalDestination(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved, filepath.Base(path)), nil
}

func (o *ChromiumOptions) CloseCrossOrigin(on bool) *ChromiumOptions {
	if on {
		o.Argument("--disable-web-security", "")
		return o.Argument("--disable-site-isolation-trials", "")
	}
	o.RemoveArgument("--disable-web-security")
	return o.RemoveArgument("--disable-site-isolation-trials")
}
