package urlpattern_test

import (
	"testing"

	"github.com/dunglas/go-urlpattern"
)

var benchmarkPatterns = []struct {
	name, pattern, baseURL string
}{
	{"simple", "https://example.com/foo/bar", ""},
	{"wildcard", "https://*.example.com/*", ""},
	{"named", "https://example.com/users/:id/posts/:postId", ""},
	{"regex", "https://example.com/items/(\\d+)", ""},
	{"full", "https://user:pw@example.com:8080/path/:id?q=:v#:frag", ""},
}

var benchmarkMatches = []struct {
	name, pattern, input string
}{
	{"simple", "https://example.com/foo/bar", "https://example.com/foo/bar"},
	{"wildcard", "https://*.example.com/*", "https://api.example.com/users/42"},
	{"named", "https://example.com/users/:id/posts/:postId", "https://example.com/users/42/posts/7"},
	{"regex", "https://example.com/items/(\\d+)", "https://example.com/items/12345"},
	{"miss", "https://example.com/foo", "https://example.com/bar"},
}

// Package-level sinks: keep return values live so the compiler cannot
// dead-code-eliminate the benchmarked calls.
var (
	benchPatternSink *urlpattern.URLPattern
	benchBoolSink    bool
	benchResultSink  *urlpattern.URLPatternResult
)

func BenchmarkNew(b *testing.B) {
	for _, bc := range benchmarkPatterns {
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			var p *urlpattern.URLPattern
			var err error
			for range b.N {
				p, err = urlpattern.New(bc.pattern, bc.baseURL, nil)
				if err != nil {
					b.Fatal(err)
				}
			}
			benchPatternSink = p
		})
	}
}

func BenchmarkTest(b *testing.B) {
	for _, bc := range benchmarkMatches {
		p, err := urlpattern.New(bc.pattern, "", nil)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			var ok bool
			for range b.N {
				ok = p.Test(bc.input, "")
			}
			benchBoolSink = ok
		})
	}
}

func BenchmarkExec(b *testing.B) {
	for _, bc := range benchmarkMatches {
		p, err := urlpattern.New(bc.pattern, "", nil)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			var r *urlpattern.URLPatternResult
			for range b.N {
				r = p.Exec(bc.input, "")
			}
			benchResultSink = r
		})
	}
}
