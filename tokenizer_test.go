package urlpattern_test

import (
	"testing"
	"time"

	"github.com/dunglas/go-urlpattern"
)

// Regression: a pattern pathname ending with a trailing backslash used to
// hang the strict tokenizer because the escape-at-end branch swallowed the
// returned error instead of propagating it, leaving the tokenizer index
// unchanged and causing an infinite loop.
func TestTrailingBackslashDoesNotHang(t *testing.T) {
	pathname := "/foo\\"
	init := &urlpattern.URLPatternInit{Pathname: &pathname}

	done := make(chan struct{})
	var newErr error
	go func() {
		defer close(done)
		_, newErr = init.New(nil)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("URLPatternInit.New hung on pathname with trailing backslash")
	}

	if newErr == nil {
		t.Fatal("expected an error for a pathname ending with a lone backslash, got nil")
	}
}
