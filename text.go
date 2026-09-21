package drissionpage

import (
	stdhtml "html"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

var spaces = regexp.MustCompile(` {2,}`)
var noText = wordSet("script style video audio iframe embed noscript canvas template")
var noWrap = wordSet("br sub sup em strong a font b span s i del ins img td th abbr bdi bdo cite code data dfn kbd mark q rp rt ruby samp small time u var wbr button slot content")
var wrapAfter = wordSet("p div h1 h2 h3 h4 h5 h6 ol li blockquote header footer address article aside main nav section figcaption summary")

func wordSet(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(s) {
		m[w] = true
	}
	return m
}
func FormatHTML(s string) string { return strings.ReplaceAll(stdhtml.UnescapeString(s), "\u00a0", " ") }

type textPart struct {
	text string
	br   bool
}

func formattedText(n *html.Node) string {
	if noText[n.Data] {
		return (&SessionElement{node: n}).RawText()
	}
	var walk func(*html.Node, bool) []textPart
	walk = func(n *html.Node, pre bool) []textPart {
		if n.Data == "br" && n.Type == html.ElementNode {
			return []textPart{{br: true}}
		}
		pre = pre || n.Data == "pre"
		if noText[n.Data] && !pre {
			return nil
		}
		var parts []textPart
		previous := ""
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.TextNode {
				txt := child.Data
				if !pre {
					if strings.Trim(txt, " \n\r\t") == "" {
						continue
					}
					txt = strings.ReplaceAll(strings.ReplaceAll(txt, "\r\n", " "), "\n", " ")
					txt = spaces.ReplaceAllString(txt, " ")
				}
				parts = append(parts, textPart{text: txt})
				continue
			}
			if child.Type != html.ElementNode {
				continue
			}
			if !noWrap[child.Data] && len(parts) > 0 && parts[len(parts)-1].text != "\n" {
				parts = append(parts, textPart{text: "\n"})
			}
			if (child.Data == "td" || child.Data == "th") && (previous == "td" || previous == "th") {
				parts = append(parts, textPart{text: "\t"})
			}
			parts = append(parts, walk(child, pre)...)
			previous = child.Data
		}
		if wrapAfter[n.Data] && len(parts) > 0 && parts[len(parts)-1].text != "\n" && !parts[len(parts)-1].br {
			parts = append(parts, textPart{text: "\n"})
		}
		return parts
	}
	parts := walk(n, false)
	if len(parts) > 0 && parts[len(parts)-1].text == "\n" {
		parts = parts[:len(parts)-1]
	}
	var out strings.Builder
	for i, p := range parts {
		if p.br {
			out.WriteByte('\n')
			continue
		}
		s := p.text
		if i+1 < len(parts) && !parts[i+1].br && strings.HasSuffix(s, " ") && strings.HasPrefix(parts[i+1].text, " ") {
			s = strings.TrimSuffix(s, " ")
		}
		out.WriteString(s)
	}
	return FormatHTML(strings.TrimSpace(out.String()))
}
