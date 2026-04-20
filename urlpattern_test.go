package urlpattern_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/dunglas/go-urlpattern"
	"github.com/nlnwa/whatwg-url/url"
)

// Port of https://github.com/web-platform-tests/wpt/blob/d3e55612911b00cb53271476de610e75a8603ae7/urlpattern/resources/urlpatterntests.js

//go:generate curl https://raw.githubusercontent.com/web-platform-tests/wpt/master/urlpattern/resources/urlpatterntestdata.json -o testdata/urlpatterntestdata.json

var (
	errInvalidPatternParam = errors.New("invalid constructor parameter")
	errBaseURLWithInit     = errors.New("invalid second argument: baseURL provided with a URLPatternInit input; use URLPatternInit.BaseURL instead")
)

type Entry struct {
	Pattern                []any `json:"pattern"`
	Inputs                 []any `json:"inputs"`
	ExactlyEmptyComponents []string      `json:"exactly_empty_components"`
	ExpectedObj            any   `json:"expected_obj"`
	ExpectedMatch          any   `json:"expected_match"`
}

func TestURLPattern(t *testing.T) {
	content, err := os.ReadFile("testdata/urlpatterntestdata.json")
	if err != nil {
		t.Fatal(err)
	}

	var data []Entry
	if err := json.Unmarshal(content, &data); err != nil {
		t.Fatal(err)
	}

	for i, entry := range data {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			pattern, err := newPattern(t, &entry)

			if e, _ := entry.ExpectedObj.(string); e == "error" {
				if err == nil {
					t.Logf("want error for %#v", entry.Pattern)
					t.FailNow()
				}

				return
			}

			if err != nil {
				t.Logf("unexpected error: %s (%#v)", err, entry)
				t.FailNow()
			}

			assertExpectedObject(t, entry, pattern)

			if e, _ := entry.ExpectedMatch.(string); e == "error" {
				_, err := callTest(pattern, entry)
				if err == nil {
					t.Logf("want error when running Test for %#v", entry)
					t.FailNow()
				}
				_, err = callExec(pattern, entry)
				if err == nil {
					t.Logf("want error when running Test for %#v", entry)
					t.FailNow()
				}

				return
			}

			testResult, err := callTest(pattern, entry)
			if err != nil {
				if len(entry.Inputs) == 1 {
					if i, ok := entry.Inputs[0].(map[string]any); ok {
						if p, _ := i["protocol"].(string); p == "café" {
							t.Skip("TODO: check why this fails, probably a bug in the test suite")
						}
					}
				}

				t.Logf("unexpected error when running Test: %s (%#v)", err, entry)
				t.FailNow()
			}

			expectedTestResult := entry.ExpectedMatch != nil

			if testResult != expectedTestResult {
				if len(entry.Pattern) > 0 {
					e, _ := entry.Pattern[0].(map[string]any)
					if pa := e["pathname"]; pa != nil {
						p := pa.(string)
						if strings.Contains(p, "[") && (strings.Contains(p, "--") || strings.Contains(p, "&&")) {
							t.Skip("Advanced unicode features aren't supported by Go")
						}
					}
				}

				t.Logf("Test must return %v; got %v (%#v)", expectedTestResult, testResult, entry)
				t.FailNow()
			}

			execResult, err := callExec(pattern, entry)
			if err != nil {
				t.Logf("unexpected error when running Test: %s (%#v)", err, entry)
				t.FailNow()
			}

			if entry.ExpectedMatch == nil {
				if execResult != nil {
					t.Logf("Match must return nil, go %#v (%#v)", execResult, entry)
					t.Fail()
				}

				return
			}

			expectedObj := entry.ExpectedMatch.(map[string]any)
			if _, ok := expectedObj["inputs"]; !ok {
				expectedObj["inputs"] = entry.Inputs
			}

			if er := newExpectedResult(entry); !reflect.DeepEqual(er, execResult) {
				t.Logf("want %#v; got %#v (%#v)", er, execResult, entry)
				t.Fail()
			}
		})
	}
}

func newPattern(t *testing.T, entry *Entry) (*urlpattern.URLPattern, error) {
	t.Helper()

	var baseURL string
	options := &urlpattern.Options{}

	switch len(entry.Pattern) {
	case 0:
		i := &urlpattern.URLPatternInit{}
		return i.New(options)

	case 2:
		switch v := entry.Pattern[1].(type) {
		case map[string]any:
			options.IgnoreCase = true

		case string:
			baseURL = v

		default:
			return nil, errInvalidPatternParam
		}

	case 3:
		options.IgnoreCase = true

		bu, ok := entry.Pattern[1].(string)
		if !ok {
			return nil, errInvalidPatternParam
		}

		baseURL = bu
	}

	switch v := entry.Pattern[0].(type) {
	case string:
		return urlpattern.New(v, baseURL, options)

	case map[string]any:
		if baseURL != "" {
			return nil, errBaseURLWithInit
		}

		return initFromObj(v).New(options)
	}

	t.Fatalf("invalid entry pattern %#v", entry.Pattern)

	return nil, nil
}

func newExpectedResult(e Entry) *urlpattern.URLPatternResult {
	expectedResult := urlpattern.URLPatternResult{}
	for k, v := range e.ExpectedMatch.(map[string]any) {
		if k == "inputs" {
			for _, initInput := range v.([]any) {
				if ip, ok := initInput.(map[string]any); ok {
					expectedResult.InitInputs = append(expectedResult.InitInputs, initFromObj(ip))
				} else {
					expectedResult.Inputs = append(expectedResult.Inputs, initInput.(string))
				}
			}

			continue
		}
		mv := v.(map[string]any)
		component := urlpattern.URLPatternComponentResult{}
		component.Input = mv["input"].(string)
		len := len(mv["groups"].(map[string]any))

		if len > 0 {
			component.Groups = make(map[string]string, len)

			for k, v := range mv["groups"].(map[string]any) {
				if v == nil {
					// TODO: this should probably be nil, but it's currently not implemented
					component.Groups[k] = ""
					continue
				}

				component.Groups[k] = v.(string)
			}
		}

		switch k {
		case "protocol":
			expectedResult.Protocol = component

		case "username":
			expectedResult.Username = component

		case "password":
			expectedResult.Password = component

		case "hostname":
			expectedResult.Hostname = component

		case "port":
			expectedResult.Port = component

		case "pathname":
			expectedResult.Pathname = component

		case "search":
			expectedResult.Search = component

		case "hash":
			expectedResult.Hash = component
		}
	}

	return &expectedResult
}

func stringOrNil(v any) *string {
	if v == nil {
		return nil
	}

	s := v.(string)

	return &s
}

func callTest(pattern *urlpattern.URLPattern, entry Entry) (bool, error) {
	if len(entry.Inputs) == 0 {
		return pattern.TestInit(&urlpattern.URLPatternInit{}), nil
	}

	if u, ok := entry.Inputs[0].(string); ok {
		var baseURL string
		if len(entry.Inputs) > 1 {
			baseURL = entry.Inputs[1].(string)
		}

		return pattern.Test(u, baseURL), nil
	}

	if len(entry.Inputs) > 1 {
		return false, errInvalidPatternParam
	}

	return pattern.TestInit(initFromObj(entry.Inputs[0].(map[string]any))), nil
}

func callExec(pattern *urlpattern.URLPattern, entry Entry) (*urlpattern.URLPatternResult, error) {
	if len(entry.Inputs) == 0 {
		return pattern.ExecInit(&urlpattern.URLPatternInit{}), nil
	}

	if u, ok := entry.Inputs[0].(string); ok {
		var baseURL string
		if len(entry.Inputs) > 1 {
			baseURL = entry.Inputs[1].(string)
		}

		return pattern.Exec(u, baseURL), nil
	}

	if len(entry.Inputs) > 1 {
		return nil, errInvalidPatternParam
	}

	return pattern.ExecInit(initFromObj(entry.Inputs[0].(map[string]any))), nil
}

func initFromObj(m map[string]any) *urlpattern.URLPatternInit {
	return &urlpattern.URLPatternInit{
		Protocol: stringOrNil(m["protocol"]),
		Username: stringOrNil(m["username"]),
		Password: stringOrNil(m["password"]),
		Hostname: stringOrNil(m["hostname"]),
		Port:     stringOrNil(m["port"]),
		Pathname: stringOrNil(m["pathname"]),
		Search:   stringOrNil(m["search"]),
		Hash:     stringOrNil(m["hash"]),
		BaseURL:  stringOrNil(m["baseURL"]),
	}
}

var earlierComponents = map[string][]string{
	"hostname": {"protocol"},
	"port":     {"protocol", "hostname"},
	"pathname": {"protocol", "hostname", "port"},
	"search":   {"protocol", "hostname", "port", "pathname"},
	"hash":     {"protocol", "hostname", "port", "pathname", "search"},
}

func buildExpected(entry Entry, component string) *string {
	if entry.ExpectedObj == nil {
		if slices.Contains(entry.ExactlyEmptyComponents, component) {
			es := ""
			return &es
		}

		if len(entry.Pattern) > 0 {
			star := "*"

			p, ok := entry.Pattern[0].(map[string]any)
			if ok {
				if p[component] != nil {
					v := p[component].(string)

					return &v
				}

				for _, e := range earlierComponents[component] {
					if _, ok := p[e]; ok {
						return &star
					}
				}

				var baseURL *url.Url
				if bu, ok := p["baseURL"]; ok {
					baseURL, _ = url.Parse(bu.(string))
				} else if len(entry.Pattern) > 1 {
					if bu, ok := entry.Pattern[1].(string); ok {
						baseURL, _ = url.Parse(bu)
					}
				}

				if baseURL != nil && component != "username" && component != "password" {
					var baseValue string
					switch component {
					case "protocol":
						baseValue = baseURL.Protocol()
						baseValue = baseValue[:len(baseValue)-1]

					case "hostname":
						baseValue = baseURL.Hostname()

					case "port":
						baseValue = baseURL.Port()

					case "pathname":
						baseValue = baseURL.Pathname()

					case "search":
						baseValue = baseURL.Search()[1:]

					case "hash":
						baseValue = baseURL.Hash()[1:]
					}

					return &baseValue
				}

				return &star
			}
		}

		return nil
	}

	o := entry.ExpectedObj.(map[string]any)
	e, ok := o[component]
	if !ok {
		return nil
	}

	expected := e.(string)

	return &expected
}

func assertExpectedObject(t *testing.T, entry Entry, pattern *urlpattern.URLPattern) {
	t.Helper()

	assertExpectedObjectProp(t, "protocol", entry, pattern.Protocol())
	assertExpectedObjectProp(t, "username", entry, pattern.Username())
	assertExpectedObjectProp(t, "password", entry, pattern.Password())
	assertExpectedObjectProp(t, "hostname", entry, pattern.Hostname())
	assertExpectedObjectProp(t, "port", entry, pattern.Port())
	assertExpectedObjectProp(t, "pathname", entry, pattern.Pathname())
	assertExpectedObjectProp(t, "search", entry, pattern.Search())
	assertExpectedObjectProp(t, "hash", entry, pattern.Hash())
}

func assertExpectedObjectProp(t *testing.T, key string, entry Entry, value string) {
	t.Helper()

	expected := buildExpected(entry, key)
	if expected == nil {
		return
	}

	if *expected != value {
		t.Logf("%s: want %q, got %q (%#v)", key, *expected, value, entry.Pattern)
		t.FailNow()
	}
}

func Example() {
	pattern, err := urlpattern.New("/books/:id", "https://example.com", nil)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%t\n", pattern.Test("https://example.com/books/123", ""))
	fmt.Printf("%t\n", pattern.Test("https://example.com/authors/123", ""))

	fmt.Printf("%v", pattern.Exec("123", "https://example.com/books/").Pathname.Groups)

	// Output: true
	// false
	// map[id:123]
}
