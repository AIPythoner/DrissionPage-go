package drissionpage

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/AIPythoner/DrissionPage-go/internal/xpath"
	"github.com/andybalholm/cascadia"
)

type Locator struct {
	Kind  string
	Value string
}

func CSS(value string) Locator      { return Locator{"css", value} }
func XPath(value string) Locator    { return Locator{"xpath", value} }
func By(kind, value string) Locator { return Locator{kind, value} }

func xpathLiteral(s string) string {
	if !strings.Contains(s, "'") {
		return "'" + s + "'"
	}
	if !strings.Contains(s, `"`) {
		return `"` + s + `"`
	}
	parts := strings.Split(s, "'")
	for i := range parts {
		parts[i] = "'" + parts[i] + "'"
	}
	return "concat(" + strings.Join(parts, `,"'",`) + ")"
}

var attrName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_.:-]*$`)
var multiAttr = regexp.MustCompile(`(@@|@\||@!)`)

func predicate(raw string) (string, error) {
	if raw == "" {
		return "not(@*)", nil
	}
	i := strings.IndexAny(raw, "=:$^")
	name := raw
	if i >= 0 {
		name = raw[:i]
	}
	expr := "@" + name
	switch name {
	case "text()", "tx()":
		expr = "."
	case "tag()", "t()":
		expr = "name()"
	default:
		if !attrName.MatchString(name) {
			return "", fmt.Errorf("%w: attribute %q", ErrInvalidLocator, name)
		}
	}
	if i < 0 {
		if expr == "." {
			return "normalize-space(text())", nil
		}
		return expr, nil
	}
	value := xpathLiteral(raw[i+1:])
	switch raw[i] {
	case '=':
		return expr + "=" + value, nil
	case ':':
		return "contains(" + expr + "," + value + ")", nil
	case '^':
		return "starts-with(" + expr + "," + value + ")", nil
	case '$':
		return "substring(" + expr + ",string-length(" + expr + ")-string-length(" + value + ")+1)=" + value, nil
	}
	return "", ErrInvalidLocator
}

// ParseLocator accepts a DrissionPage string or a typed By/CSS/XPath locator.
// Bare strings use Chromium's native search; static HTML uses text containment.
func ParseLocator(value any) (Locator, error) {
	var loc Locator
	switch v := value.(type) {
	case Locator:
		switch strings.ToLower(v.Kind) {
		case "css", "css selector":
			loc = CSS(v.Value)
		case "xpath":
			loc = XPath(v.Value)
		case "id", "name", "class name":
			name := v.Kind
			if name == "class name" {
				name = "class"
			}
			loc = XPath("//*[@" + name + "=" + xpathLiteral(v.Value) + "]")
		case "tag name":
			loc = XPath("//*[name()=" + xpathLiteral(v.Value) + "]")
		case "link text":
			loc = XPath("//a[text()=" + xpathLiteral(v.Value) + "]")
		case "partial link text":
			loc = XPath("//a[contains(text()," + xpathLiteral(v.Value) + ")]")
		default:
			return loc, fmt.Errorf("%w: %s", ErrInvalidLocator, v.Kind)
		}
	case string:
		s := v
		for _, pair := range [][2]string{{"t:", "tag:"}, {"t=", "tag="}, {"tx:", "text:"}, {"tx=", "text="}, {"tx^", "text^"}, {"tx$", "text$"}, {"c:", "css:"}, {"c=", "css="}, {"x:", "xpath:"}, {"x=", "xpath="}} {
			if strings.HasPrefix(s, pair[0]) {
				s = pair[1] + s[len(pair[0]):]
				break
			}
		}
		if len(s) > 1 && (s[0] == '#' || s[0] == '.') && strings.ContainsRune("=:$^", rune(s[1])) {
			if s[0] == '#' {
				s = "@id" + s[1:]
			} else {
				s = "@class" + s[1:]
			}
		}
		switch {
		case strings.HasPrefix(s, "ax:"), strings.HasPrefix(s, "ax="):
			loc = Locator{"ax", s[3:]}
		case strings.HasPrefix(s, "css:"), strings.HasPrefix(s, "css="):
			loc = CSS(s[4:])
		case strings.HasPrefix(s, "xpath:"), strings.HasPrefix(s, "xpath="):
			loc = XPath(s[6:])
		case strings.HasPrefix(s, "text") && len(s) >= 5 && strings.ContainsRune("=:$^", rune(s[4])):
			if s[4] == '=' {
				loc = XPath("//*[text()=" + xpathLiteral(s[5:]) + "]")
			} else {
				p, e := predicate("text()" + s[4:])
				if e != nil {
					return loc, e
				}
				loc = XPath("//*/text()[" + p + "]/..")
			}
		case strings.HasPrefix(s, "@"), strings.HasPrefix(s, "tag:"), strings.HasPrefix(s, "tag="):
			tag, attrs := "", s
			if strings.HasPrefix(s, "tag") {
				attrs = ""
				tag = s[4:]
				if i := strings.IndexByte(tag, '@'); i >= 0 {
					attrs = tag[i:]
					tag = tag[:i]
				}
			}
			var predicates []string
			if attrs != "" {
				if strings.Contains(attrs, "@@") && strings.Contains(attrs, "@|") {
					return loc, fmt.Errorf("%w: cannot mix @@ and @|", ErrInvalidLocator)
				}
				if strings.HasPrefix(attrs, "@@") || strings.HasPrefix(attrs, "@|") || strings.HasPrefix(attrs, "@!") {
					matches := multiAttr.FindAllStringIndex(attrs, -1)
					for j, m := range matches {
						end := len(attrs)
						if j+1 < len(matches) {
							end = matches[j+1][0]
						}
						p, e := predicate(attrs[m[1]:end])
						if e != nil {
							return loc, e
						}
						if attrs[m[0]:m[1]] == "@!" {
							p = "not(" + p + ")"
						}
						predicates = append(predicates, p)
					}
				} else {
					p, e := predicate(attrs[1:])
					if e != nil {
						return loc, e
					}
					predicates = append(predicates, p)
				}
			}
			join := " and "
			if strings.Contains(attrs, "@|") {
				join = " or "
			}
			expr := strings.Join(predicates, join)
			if tag != "" {
				t := "name()=" + xpathLiteral(tag)
				if expr != "" {
					expr = t + " and (" + expr + ")"
				} else {
					expr = t
				}
			}
			loc = XPath("//*")
			if expr != "" {
				loc.Value += "[" + expr + "]"
			}
		case s == "":
			loc = XPath("//*")
		default:
			loc = Locator{"search", s}
		}
	default:
		return loc, fmt.Errorf("%w: expected string or Locator", ErrInvalidLocator)
	}
	if loc.Kind == "css" {
		if _, e := cascadia.Compile(loc.Value); e != nil {
			return loc, fmt.Errorf("%w: %v", ErrInvalidLocator, e)
		}
	}
	if loc.Kind == "xpath" {
		if _, e := xpath.Compile(loc.Value); e != nil {
			return loc, fmt.Errorf("%w: %v", ErrInvalidLocator, e)
		}
	}
	return loc, nil
}
