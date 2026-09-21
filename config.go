package drissionpage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// OptionsManager reads/writes INI without executing Python literals. Unknown
// sections and values survive a load/save cycle.
type OptionsManager struct{ Sections map[string]map[string]string }

func ReadINI(r io.Reader) (*OptionsManager, error) {
	m := &OptionsManager{Sections: map[string]map[string]string{}}
	section := ""
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), 4<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			if m.Sections[section] == nil {
				m.Sections[section] = map[string]string{}
			}
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 || section == "" {
			return nil, fmt.Errorf("invalid INI line: %q", line)
		}
		m.Sections[section][strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return m, scanner.Err()
}
func LoadINI(path string) (*OptionsManager, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	return ReadINI(f)
}
func (m *OptionsManager) Get(section, key string) string { return m.Sections[section][key] }
func (m *OptionsManager) Set(section, key, value string) {
	if m.Sections == nil {
		m.Sections = map[string]map[string]string{}
	}
	if m.Sections[section] == nil {
		m.Sections[section] = map[string]string{}
	}
	m.Sections[section][key] = value
}
func (m *OptionsManager) Save(path string) error {
	var b strings.Builder
	sections := make([]string, 0, len(m.Sections))
	for s := range m.Sections {
		sections = append(sections, s)
	}
	sort.Strings(sections)
	for _, s := range sections {
		fmt.Fprintf(&b, "[%s]\n", s)
		keys := make([]string, 0, len(m.Sections[s]))
		for k := range m.Sections[s] {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "%s = %s\n", k, m.Sections[s][k])
		}
		b.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(b.String()), 0600)
}

// literalJSON handles Python's string/list/dict/bool/None configuration syntax.
// Function calls, identifiers, and other executable expressions are rejected.
func literalJSON(text string) ([]byte, error) {
	var out strings.Builder
	for i := 0; i < len(text); {
		c := text[i]
		if c == '\'' || c == '"' {
			quote := c
			i++
			var value strings.Builder
			closed := false
			for i < len(text) {
				c = text[i]
				i++
				if c == quote {
					closed = true
					break
				}
				if c != '\\' {
					value.WriteByte(c)
					continue
				}
				if i == len(text) {
					return nil, fmt.Errorf("unfinished escape")
				}
				c = text[i]
				i++
				switch c {
				case '\\', '\'', '"':
					value.WriteByte(c)
				case 'n':
					value.WriteByte('\n')
				case 'r':
					value.WriteByte('\r')
				case 't':
					value.WriteByte('\t')
				case 'b':
					value.WriteByte('\b')
				case 'f':
					value.WriteByte('\f')
				case 'u', 'x':
					n := 4
					if c == 'x' {
						n = 2
					}
					if i+n > len(text) {
						return nil, fmt.Errorf("unfinished unicode escape")
					}
					v, e := strconv.ParseUint(text[i:i+n], 16, 32)
					if e != nil {
						return nil, e
					}
					value.WriteRune(rune(v))
					i += n
				default:
					value.WriteByte('\\')
					value.WriteByte(c)
				}
			}
			if !closed {
				return nil, fmt.Errorf("unterminated string")
			}
			encoded, _ := json.Marshal(value.String())
			out.Write(encoded)
			continue
		}
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' {
			j := i
			for i < len(text) && ((text[i] >= 'a' && text[i] <= 'z') || (text[i] >= 'A' && text[i] <= 'Z')) {
				i++
			}
			switch text[j:i] {
			case "True", "true":
				out.WriteString("true")
			case "False", "false":
				out.WriteString("false")
			case "None", "null":
				out.WriteString("null")
			default:
				return nil, fmt.Errorf("unsupported configuration literal %q", text[j:i])
			}
			continue
		}
		// Python tuples of arguments can also be treated as arrays.
		if c == '(' {
			c = '['
		}
		if c == ')' {
			c = ']'
		}
		out.WriteByte(c)
		i++
	}
	data := []byte(out.String())
	if !json.Valid(data) {
		return nil, fmt.Errorf("invalid configuration literal")
	}
	return data, nil
}
func (m *OptionsManager) Decode(section, key string, target any) error {
	s := m.Get(section, key)
	if s == "" {
		return nil
	}
	data, e := literalJSON(s)
	if e != nil {
		return fmt.Errorf("[%s] %s: %w", section, key, e)
	}
	return json.Unmarshal(data, target)
}
func (m *OptionsManager) ChromiumOptions() (*ChromiumOptions, error) {
	o := NewChromiumOptions()
	o.BrowserPath = m.Get("chromium_options", "browser_path")
	o.Address = m.Get("chromium_options", "address")
	o.LoadMode = m.Get("chromium_options", "load_mode")
	if o.LoadMode == "" {
		o.LoadMode = "normal"
	}
	if err := m.Decode("chromium_options", "extensions", &o.Extensions); err != nil {
		return nil, err
	}
	if e := m.Decode("chromium_options", "arguments", &o.Arguments); e != nil {
		return nil, e
	}
	if user := m.Get("chromium_options", "user"); user != "" {
		o.Argument("--profile-directory", user)
	}
	if e := m.Decode("chromium_options", "prefs", &o.Preferences); e != nil {
		return nil, e
	}
	for _, arg := range o.Arguments {
		if strings.HasPrefix(arg, "--headless") {
			o.Headless = true
		}
		if strings.HasPrefix(arg, "--user-data-dir=") {
			o.UserDataPath = strings.TrimPrefix(arg, "--user-data-dir=")
		}
	}
	if err := m.duration("timeouts", "base", &o.Timeout); err != nil {
		return nil, err
	}
	o.Proxy = m.Get("proxies", "http")
	if o.Proxy == "" {
		o.Proxy = m.Get("proxies", "https")
	}
	if err := m.duration("timeouts", "page_load", &o.PageLoadTimeout); err != nil {
		return nil, err
	}
	if err := m.duration("timeouts", "script", &o.ScriptTimeout); err != nil {
		return nil, err
	}
	if err := m.retry(&o.RetryTimes, &o.RetryInterval); err != nil {
		return nil, err
	}
	return o, nil
}
func (m *OptionsManager) SessionOptions() (*SessionOptions, error) {
	o := NewSessionOptions()
	var headers map[string]string
	if e := m.Decode("session_options", "headers", &headers); e != nil {
		return nil, e
	}
	for k, v := range headers {
		o.Headers.Set(k, v)
	}
	o.Proxy = m.Get("proxies", "http")
	if o.Proxy == "" {
		o.Proxy = m.Get("proxies", "https")
	}
	if err := m.duration("timeouts", "base", &o.Timeout); err != nil {
		return nil, err
	}
	if err := m.duration("session_options", "timeout", &o.Timeout); err != nil {
		return nil, err
	}
	if err := m.retry(&o.RetryTimes, &o.RetryInterval); err != nil {
		return nil, err
	}
	return o, nil
}

func (m *OptionsManager) duration(section, key string, target *time.Duration) error {
	value := m.Get(section, key)
	if value == "" {
		return nil
	}
	seconds, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("[%s] %s: %w", section, key, err)
	}
	if seconds < 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds >= float64(math.MaxInt64)/float64(time.Second) {
		return fmt.Errorf("[%s] %s must be a finite nonnegative duration", section, key)
	}
	*target = time.Duration(seconds * float64(time.Second))
	return nil
}
func (m *OptionsManager) retry(times *int, interval *time.Duration) error {
	if value := m.Get("others", "retry_times"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		if n < 0 {
			return fmt.Errorf("negative retry count")
		}
		*times = n
	}
	return m.duration("others", "retry_interval", interval)
}
