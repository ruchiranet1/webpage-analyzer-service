package analyzer

import (
	"strings"

	"golang.org/x/net/html"
)

func detectHTMLVersion(doc *html.Node) string {
	for n := doc; n != nil; n = n.Parent {
		if n.Type == html.DoctypeNode {
			d := strings.ToLower(n.Data)
			if strings.Contains(d, "html") && !strings.Contains(d, "xhtml") {
				return "HTML5"
			}
			if strings.Contains(d, "xhtml") {
				return "XHTML"
			}
			if strings.Contains(d, "4.01") {
				return "HTML 4.01"
			}
			return "HTML 4.01"
		}
	}
	return "HTML5"
}

func getTitle(doc *html.Node) string {
	var f func(*html.Node) string
	f = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "title" && n.FirstChild != nil {
			return strings.TrimSpace(n.FirstChild.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if t := f(c); t != "" {
				return t
			}
		}
		return ""
	}
	return f(doc)
}

func countHeadings(doc *html.Node) map[string]int {
	counts := make(map[string]int)
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && strings.HasPrefix(n.Data, "h") {
			if len(n.Data) == 2 && n.Data[1] >= '1' && n.Data[1] <= '6' {
				counts[n.Data]++
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return counts
}
