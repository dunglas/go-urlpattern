// Package fuzz holds the OSS-Fuzz harness in its own directory so its
// FuzzXxx functions live in an internal (non "_test") package, which is
// what go-118-fuzz-build requires to find them.
package fuzz

import (
	"testing"

	"github.com/dunglas/go-urlpattern"
)

func FuzzNew(f *testing.F) {
	patterns := []string{
		"", "*", "/foo", "/foo/*", "/foo/**", "/foo/*+", "/foo/*?",
		"/foo/:bar", "/foo/:bar?", "/foo/:bar+", "/foo/:bar*", "/foo/:bar(.*)",
		"/:foo((?<x>a))", "/:foo\\bar", "/:foo.", "/:foo..", "/:café", "/:℘", "/:㐀",
		"/foo/(.*)", "/foo/(.*)*", "/foo/(.*)+", "/foo/(.*)?",
		"/foo/(bar(?<x>baz))", "/foo/([^\\/]+?)", "/([[a-z]--a])", "/([\\d&&[0-1]])",
		"(café)://foo", "(https://)example.com/foo", "(https|javascript)", "(data|javascript)",
		"*{}**?", "*\\:1]", "*/{*}", "*\\/*", "/foo\\", "/foo(", "/foo!",
		"{", "}", "[", "]", "\\", "%", "://", "http://example.com/:id",
		"/foo?bar#baz", "../foo", "./foo/bar", "/caf%C3%A9",
	}
	baseURLs := []string{"", "https://example.com/", "https://example.com/foo/bar", "not-a-url"}

	for _, p := range patterns {
		for _, b := range baseURLs {
			f.Add(p, b)
		}
	}

	f.Fuzz(func(t *testing.T, pattern, baseURL string) {
		p, err := urlpattern.New(pattern, baseURL, nil)
		if err != nil || p == nil {
			return
		}

		_ = p.Protocol()
		_ = p.Username()
		_ = p.Password()
		_ = p.Hostname()
		_ = p.Port()
		_ = p.Pathname()
		_ = p.Search()
		_ = p.Hash()
		_ = p.HasRegexpGroups()

		_ = p.Test(pattern, baseURL)
		_ = p.Exec(pattern, baseURL)
	})
}

func FuzzURLPatternInit(f *testing.F) {
	strs := []string{
		"", "*", "/foo", "/foo/*", "/foo/:bar", "/foo/:bar?", "/foo/(.*)",
		"café", "℘", "㐀", "[::1]", "example.com", "8080", "?q=1", "#frag",
	}

	for _, s := range strs {
		f.Add(s, s, s, s, s)
	}

	f.Fuzz(func(t *testing.T, protocol, hostname, port, pathname, search string) {
		init := &urlpattern.URLPatternInit{
			Protocol: &protocol,
			Hostname: &hostname,
			Port:     &port,
			Pathname: &pathname,
			Search:   &search,
		}

		p, err := init.New(nil)
		if err != nil || p == nil {
			return
		}

		_ = p.TestInit(init)
		_ = p.ExecInit(init)
	})
}
