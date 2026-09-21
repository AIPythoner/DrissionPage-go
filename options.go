package drissionpage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (o *ChromiumOptions) SetRetry(times int, interval time.Duration) *ChromiumOptions {
	o.RetryTimes = times
	o.RetryInterval = interval
	return o
}

func (o *ChromiumOptions) RemoveArgument(name string) *ChromiumOptions {
	out := o.Arguments[:0]
	for _, arg := range o.Arguments {
		if arg != name && !strings.HasPrefix(arg, name+"=") {
			out = append(out, arg)
		}
	}
	o.Arguments = out
	return o
}
func (o *ChromiumOptions) Argument(name, value string) *ChromiumOptions {
	o.RemoveArgument(name)
	if value != "" {
		name += "=" + value
	}
	return o.SetArgument(name)
}
func (o *ChromiumOptions) SetUserDataPath(path string) *ChromiumOptions {
	o.UserDataPath = path
	return o
}
func (o *ChromiumOptions) SetProxy(proxy string) *ChromiumOptions   { o.Proxy = proxy; return o }
func (o *ChromiumOptions) SetLoadMode(mode string) *ChromiumOptions { o.LoadMode = mode; return o }
func (o *ChromiumOptions) SetTimeouts(base, load, script time.Duration) *ChromiumOptions {
	o.Timeout = base
	o.PageLoadTimeout = load
	o.ScriptTimeout = script
	return o
}
func (o *ChromiumOptions) AddExtension(path string) *ChromiumOptions {
	o.Extensions = append(o.Extensions, path)
	return o
}
func (o *ChromiumOptions) SetPref(key string, value any) *ChromiumOptions {
	if o.Preferences == nil {
		o.Preferences = map[string]any{}
	}
	o.Preferences[key] = value
	return o
}
func (o *ChromiumOptions) NoImages(on bool) *ChromiumOptions {
	if on {
		return o.Argument("--blink-settings", "imagesEnabled=false")
	}
	return o.RemoveArgument("--blink-settings")
}
func (o *ChromiumOptions) NoJS(on bool) *ChromiumOptions {
	if on {
		return o.SetPref("profile.default_content_setting_values.javascript", 2)
	}
	return o.SetPref("profile.default_content_setting_values.javascript", 1)
}
func (o *ChromiumOptions) Mute(on bool) *ChromiumOptions {
	if on {
		return o.Argument("--mute-audio", "")
	}
	return o.RemoveArgument("--mute-audio")
}
func (o *ChromiumOptions) SetUserAgent(ua string) *ChromiumOptions {
	return o.Argument("--user-agent", ua)
}
func (o *ChromiumOptions) DisablePDFPreview(on bool) *ChromiumOptions {
	return o.SetPref("plugins.always_open_pdf_externally", on)
}
func (o *SessionOptions) SetHeader(name, value string) *SessionOptions {
	if o.Headers == nil {
		o.Headers = make(map[string][]string)
	}
	o.Headers.Set(name, value)
	return o
}
func (o *SessionOptions) SetTimeout(timeout time.Duration) *SessionOptions {
	o.Timeout = timeout
	return o
}
func (o *SessionOptions) SetRetry(times int, interval time.Duration) *SessionOptions {
	o.RetryTimes = times
	o.RetryInterval = interval
	return o
}

func browserPreferences(o ChromiumOptions) ([]byte, error) {
	prefs := map[string]any{}
	if o.UserDataPath != "" {
		profile := "Default"
		for _, arg := range o.Arguments {
			if strings.HasPrefix(arg, "--profile-directory=") {
				profile = strings.TrimPrefix(arg, "--profile-directory=")
			}
		}
		data, e := os.ReadFile(filepath.Join(o.UserDataPath, profile, "Preferences"))
		if e == nil {
			if e = json.Unmarshal(data, &prefs); e != nil {
				return nil, e
			}
		} else if !os.IsNotExist(e) {
			return nil, e
		}
	}
	keys := make([]string, 0, len(o.Preferences))
	for k := range o.Preferences {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts := strings.Split(key, ".")
		current := prefs
		for _, part := range parts[:len(parts)-1] {
			child, ok := current[part].(map[string]any)
			if !ok {
				child = map[string]any{}
				current[part] = child
			}
			current = child
		}
		current[parts[len(parts)-1]] = o.Preferences[key]
	}
	return json.Marshal(prefs)
}
