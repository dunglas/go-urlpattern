// Package urlpattern implements the URLPattern web API.
//
// The specification is available at https://wicg.github.io/urlpattern/.
package urlpattern

// https://wicg.github.io/urlpattern/#urlpattern
type URLPattern struct {
	Protocol string
	Username string
	Password string
	Hostname string
	Port     string
	Pathname string
	Search   string
	Hash     string
}

// https://wicg.github.io/urlpattern/#parse-a-constructor-string
func New(input string, baseURL string) {

}

// https://wicg.github.io/urlpattern/#dictdef-urlpatterninit
type urlPatternInit struct {
	URLPattern
	baseURL string
}

// TODO: UrlPatternResult
// TODO: URLPatternComponentResult
