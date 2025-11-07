package analyzer

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/net/html"
)

type Link struct {
	URL        *url.URL
	IsInternal bool
}

func extractLinks(doc *html.Node, baseURLStr string) []Link {
	baseURL, _ := url.Parse(baseURLStr)
	var links []Link
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "a" || n.Data == "link") {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					u, err := url.Parse(attr.Val)
					if err != nil {
						continue
					}
					abs := baseURL.ResolveReference(u)
					isInternal := abs.Host == baseURL.Host
					links = append(links, Link{URL: abs, IsInternal: isInternal})
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return links
}

func checkLinksConcurrently(ctx context.Context, links []Link) (int, int, int) {
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10)
	results := make(chan struct{ internal, external, inaccessible int }, len(links))

	unique := map[string]bool{}
	count := 0
	for _, l := range links {
		key := l.URL.String()
		if !unique[key] && count < 50 {
			unique[key] = true
			count++
			wg.Add(1)
			go func(link Link) {
				defer wg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()

				accessible := isLinkAccessible(ctx, link.URL)
				i, e, ia := 0, 0, 0
				if accessible {
					if link.IsInternal {
						i = 1
					} else {
						e = 1
					}
				} else {
					ia = 1
				}
				results <- struct{ internal, external, inaccessible int }{i, e, ia}
			}(l)
		}
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	internal, external, inaccessible := 0, 0, 0
	for r := range results {
		internal += r.internal
		external += r.external
		inaccessible += r.inaccessible
	}
	return internal, external, inaccessible
}

func isLinkAccessible(ctx context.Context, u *url.URL) bool {
	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, u.String(), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}
