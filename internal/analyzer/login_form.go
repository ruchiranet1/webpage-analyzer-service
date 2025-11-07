package analyzer

import (
	"strings"

	"golang.org/x/net/html"
)

func hasLoginForm(doc *html.Node) bool {
	var check func(*html.Node) bool
	check = func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == "input" {
			for _, attr := range n.Attr {
				val := strings.ToLower(attr.Val)
				key := strings.ToLower(attr.Key)
				if key == "type" && val == "password" {
					return true
				}
				if strings.Contains(val, "password") ||
					strings.Contains(val, "login") ||
					strings.Contains(val, "username") ||
					strings.Contains(val, "email") {
					return true
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if check(c) {
				return true
			}
		}
		return false
	}
	return check(doc)
}
