// Example: Concurrent web crawler with nested scopes.
//
// Demonstrates Scope.Child(), Map, and bounded concurrency.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/cagatay35/conquer"
)

type Page struct {
	URL   string
	Title string
	Links []string
}

func crawlPage(_ context.Context, url string) (*Page, error) {
	time.Sleep(time.Duration(rand.Intn(30)) * time.Millisecond)

	links := []string{
		fmt.Sprintf("%s/about", url),
		fmt.Sprintf("%s/contact", url),
	}

	return &Page{
		URL:   url,
		Title: fmt.Sprintf("Title of %s", url),
		Links: links,
	}, nil
}

func main() {
	ctx := context.Background()
	seeds := []string{
		"https://example.com",
		"https://example.org",
		"https://example.net",
	}

	var mu sync.Mutex
	visited := make(map[string]bool)
	var pages []*Page

	err := conquer.Run(ctx, func(s *conquer.Scope) error {
		results, err := conquer.Map(ctx, seeds, func(ctx context.Context, url string) (*Page, error) {
			return crawlPage(ctx, url)
		}, conquer.WithMaxGoroutines(5))
		if err != nil {
			return err
		}

		for _, page := range results {
			mu.Lock()
			visited[page.URL] = true
			pages = append(pages, page)
			mu.Unlock()
		}

		s.Child(func(cs *conquer.Scope) error {
			for _, page := range results {
				for _, link := range page.Links {
					link := link
					mu.Lock()
					alreadyVisited := visited[link]
					if !alreadyVisited {
						visited[link] = true
					}
					mu.Unlock()

					if alreadyVisited {
						continue
					}

					cs.Go(func(ctx context.Context) error {
						subPage, err := crawlPage(ctx, link)
						if err != nil {
							return err
						}
						mu.Lock()
						pages = append(pages, subPage)
						mu.Unlock()
						return nil
					})
				}
			}
			return nil
		}, conquer.WithMaxGoroutines(3))

		return nil
	}, conquer.WithTimeout(10*time.Second))

	if err != nil {
		log.Fatalf("Crawl error: %v", err)
	}

	fmt.Printf("Crawled %d pages:\n", len(pages))
	for _, p := range pages {
		fmt.Printf("  - %s: %s\n", p.URL, p.Title)
	}
}
