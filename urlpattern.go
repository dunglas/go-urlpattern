// Package urlpattern implements the URLPattern web API.
//
// The specification is available at https://urlpattern.spec.whatwg.org/.
package urlpattern

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

var (
	NoBaseURLError             = errors.New("relative URL and no baseURL provided")
	UnexpectedEmptyStringError = errors.New("unexpected empty string")
)

// https://url.spec.whatwg.org/#special-scheme
var specialSchemeList = []string{"ftp", "file", "http", "https", "ws", "wss"}

// https://urlpattern.spec.whatwg.org/#urlpattern
type URLPattern struct {
	Protocol *string
	Username *string
	Password *string
	Hostname *string
	Port     *string
	Pathname *string
	Search   *string
	Hash     *string
}

type URLPatternResult struct {
	Inputs []string

	Protocol URLPatternComponentResult
	Username URLPatternComponentResult
	Password URLPatternComponentResult
	Hostname URLPatternComponentResult
	Port     URLPatternComponentResult
	Pathname URLPatternComponentResult
	Search   URLPatternComponentResult
	Hash     URLPatternComponentResult
}

type URLPatternComponentResult struct {
	Input  string
	Groups map[string]string
}

// https://urlpattern.spec.whatwg.org/#url-pattern-struct
type urlPatternStruct struct {
	protocol *component
	username *component
	password *component
	hostname *component
	port     *component
	pathname *component
	search   *component
	hash     *component
}

// https://urlpattern.spec.whatwg.org/#component
type component struct {
	patternString     string
	regularExpression *regexp.Regexp
	groupNameList     []string
	hasRegexpGroups   bool
}

// https://urlpattern.spec.whatwg.org/#protocol-component-matches-a-special-scheme
func (c *component) protocolComponentMatchesSpecialScheme() bool {
	for _, scheme := range specialSchemeList {
		if c.regularExpression.MatchString(scheme) {
			return true
		}
	}

	return false
}

// https://urlpattern.spec.whatwg.org/#url-pattern-create
func New(input string, baseURL string, options Options) (*urlPatternStruct, error) {
	init, err := parseConstructorString(input)
	if err != nil {
		return nil, err
	}

	if baseURL == "" && *init.Protocol == "" {
		return nil, NoBaseURLError
	}

	if baseURL != "" {
		init.baseURL = &baseURL
	}

	return NewFromURLPatternInit(init, options)
}

// https://urlpattern.spec.whatwg.org/#url-pattern-create
func NewFromURLPatternInit(init *urlPatternInit, opt Options) (*urlPatternStruct, error) {
	processedInit, err := init.process("pattern", nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, err
	}

	star := "*"
	if processedInit.Protocol == nil {
		processedInit.Protocol = &star
	}
	if processedInit.Username == nil {
		processedInit.Username = &star
	}
	if processedInit.Password == nil {
		processedInit.Password = &star
	}
	if processedInit.Hostname == nil {
		processedInit.Hostname = &star
	}
	if processedInit.Port == nil {
		processedInit.Port = &star
	}
	if processedInit.Pathname == nil {
		processedInit.Pathname = &star
	}
	if processedInit.Search == nil {
		processedInit.Search = &star
	}
	if processedInit.Hash == nil {
		processedInit.Hash = &star
	}

	var emptyString string
	for _, s := range specialSchemeList {
		if *processedInit.Protocol == s && *processedInit.Port == DefaultPorts[s] {
			processedInit.Protocol = &emptyString
			break
		}
	}

	defaultOptions := options{}

	urlPattern := &urlPatternStruct{}
	urlPattern.protocol, err = compileComponent(*processedInit.Protocol, canonicalizeProtocol, defaultOptions)
	if err != nil {
		return nil, err
	}
	urlPattern.username, err = compileComponent(*processedInit.Username, canonicalizeUsername, defaultOptions)
	if err != nil {
		return nil, err
	}

	urlPattern.password, err = compileComponent(*processedInit.Password, canonicalizePassword, defaultOptions)
	if err != nil {
		return nil, err
	}

	// If the result running hostname pattern is an IPv6 address given processedInit["hostname"] is true, then set urlPattern’s hostname component to the result of compiling a component given processedInit["hostname"], canonicalize an IPv6 hostname, and hostname options.

	hostnameOptions := options{delimiterCodePoint: '.'}
	if hostnamePatternIsIPv6Address(*processedInit.Hostname) {
		urlPattern.hostname, err = compileComponent(*processedInit.Hostname, canonicalizeIPv6Hostname, hostnameOptions)
		if err != nil {
			return nil, err
		}
	} else {
		urlPattern.hostname, err = compileComponent(*processedInit.Hostname, canonicalizeHostname, hostnameOptions)
		if err != nil {
			return nil, err
		}
	}

	urlPattern.port, err = compileComponent(*processedInit.Port, func(s string) (string, error) { return canonicalizePort(s, "") }, defaultOptions)
	if err != nil {
		return nil, err
	}

	urlPattern.port, err = compileComponent(*processedInit.Port, func(s string) (string, error) { return canonicalizePort(s, "") }, defaultOptions)
	if err != nil {
		return nil, err
	}

	compileOptions := defaultOptions
	compileOptions.ignoreCase = opt.ignoreCase

	pathnameOptions := options{'/', '/', false}

	if urlPattern.protocol.protocolComponentMatchesSpecialScheme() {
		pathCompileOptions := pathnameOptions
		pathCompileOptions.ignoreCase = opt.ignoreCase

		urlPattern.pathname, err = compileComponent(*processedInit.Pathname, canonicalizePathname, pathCompileOptions)
		if err != nil {
			return nil, err
		}
	} else {
		urlPattern.pathname, err = compileComponent(*processedInit.Pathname, canonicalizeOpaquePathname, compileOptions)
		if err != nil {
			return nil, err
		}
	}

	urlPattern.search, err = compileComponent(*processedInit.Search, canonicalizeSearch, compileOptions)
	if err != nil {
		return nil, err
	}

	urlPattern.hash, err = compileComponent(*processedInit.Hash, canonicalizeHash, compileOptions)
	if err != nil {
		return nil, err
	}

	return urlPattern, nil
}

// https://urlpattern.spec.whatwg.org/#url-pattern-match
func (u *urlPatternStruct) Match(input, baseURLString string) *URLPatternResult {
	protocol := ""
	username := ""
	password := ""
	hostname := ""
	port := ""
	pathname := ""
	search := ""
	hash := ""

	inputs := []string{input}

	var baseURL *url.URL
	var err error

	if baseURLString != "" {
		baseURL, err = url.Parse(baseURLString)
		if err != nil {
			return nil
		}

		inputs = append(inputs, baseURLString)
	}

	ur, err := url.Parse(input)
	if err != nil {
		return nil
	}

	if baseURL != nil {
		ur.ResolveReference(baseURL)
	}

	protocol = ur.Scheme
	username = ur.User.Username()
	password, _ = ur.User.Password()
	hostname = ur.Hostname()
	port = ur.Port()
	pathname = ur.EscapedPath()
	search = ur.Query().Encode()
	hash = ur.EscapedFragment()

	protocolExecResult := u.protocol.regularExpression.FindStringSubmatch(protocol)
	usernameExecResult := u.username.regularExpression.FindStringSubmatch(username)
	passwordExecResult := u.password.regularExpression.FindStringSubmatch(password)
	hostnameExecResult := u.hostname.regularExpression.FindStringSubmatch(hostname)
	portExecResult := u.port.regularExpression.FindStringSubmatch(port)
	pathnameExecResult := u.pathname.regularExpression.FindStringSubmatch(pathname)
	searchExecResult := u.search.regularExpression.FindStringSubmatch(search)
	hashExecResult := u.hash.regularExpression.FindStringSubmatch(hash)

	if protocolExecResult == nil &&
		usernameExecResult == nil &&
		passwordExecResult == nil &&
		hostnameExecResult == nil &&
		portExecResult == nil &&
		pathnameExecResult == nil &&
		searchExecResult == nil &&
		hashExecResult == nil {
		return nil
	}

	result := &URLPatternResult{Inputs: inputs}
	result.Protocol = createComponentMatchResult(*u.protocol, protocol, protocolExecResult)
	result.Username = createComponentMatchResult(*u.username, username, usernameExecResult)
	result.Password = createComponentMatchResult(*u.password, password, passwordExecResult)
	result.Hostname = createComponentMatchResult(*u.hostname, hostname, hostnameExecResult)
	result.Port = createComponentMatchResult(*u.port, port, portExecResult)
	result.Pathname = createComponentMatchResult(*u.pathname, pathname, pathnameExecResult)
	result.Search = createComponentMatchResult(*u.search, search, searchExecResult)
	result.Hash = createComponentMatchResult(*u.hash, hash, hashExecResult)

	return result
}

func (u *urlPatternStruct) Test(input, baseURL string) bool {
	return u.Match(input, baseURL) != nil
}

// https://urlpattern.spec.whatwg.org/#url-pattern-has-regexp-groups
func (u *urlPatternStruct) HasRegexpGroups() bool {
	return u.protocol.hasRegexpGroups ||
		u.username.hasRegexpGroups ||
		u.password.hasRegexpGroups ||
		u.hostname.hasRegexpGroups ||
		u.port.hasRegexpGroups ||
		u.pathname.hasRegexpGroups ||
		u.search.hasRegexpGroups ||
		u.hash.hasRegexpGroups
}

// https://urlpattern.spec.whatwg.org/#create-a-component-match-result
func createComponentMatchResult(component component, input string, execResult []string) URLPatternComponentResult {
	result := URLPatternComponentResult{Input: input, Groups: make(map[string]string, len(execResult))}

	for index := 1; index < len(execResult); index++ {
		name := component.groupNameList[index-1]
		value := execResult[index]

		result.Groups[name] = value
	}

	return result
}

type Options struct {
	ignoreCase bool
}

// https://urlpattern.spec.whatwg.org/#dictdef-urlpatterninit
type urlPatternInit struct {
	URLPattern
	baseURL *string
}

// https://urlpattern.spec.whatwg.org/#process-a-urlpatterninit
func (init *urlPatternInit) process(iType string, protocol, username, password, hostname, port, pathname, search, hash *string) (*urlPatternInit, error) {
	result := &urlPatternInit{
		URLPattern: URLPattern{protocol, username, password, hostname, port, pathname, search, hash},
	}

	var (
		baseURL *url.URL
		err     error
	)
	if init.baseURL != nil {
		baseURL, err = url.Parse(*init.baseURL)
		if err != nil {
			return nil, err
		}

		if init.Protocol == nil {
			p := processBaseURLString(baseURL.Scheme, iType)
			result.Protocol = &p
		}

		// TODO: the end of this block can be simplified, but let's be as close as possible from the standard algorithm for now

		if iType != "pattern" && init.Protocol == nil && init.Hostname == nil && init.Port == nil && init.Username == nil {
			u := processBaseURLString(baseURL.User.Username(), iType)
			result.Username = &u
		}

		if iType != "pattern" && init.Protocol == nil && init.Hostname == nil && init.Port == nil && init.Username == nil && init.Password == nil {
			password, _ := baseURL.User.Password()
			p := processBaseURLString(password, iType)
			result.Password = &p
		}

		if init.Protocol == nil && init.Hostname == nil {
			baseHost := baseURL.Host
			h := processBaseURLString(baseHost, iType)
			result.Hostname = &h
		}

		if init.Protocol == nil && init.Hostname == nil && init.Port == nil {
			p := baseURL.Port()
			result.Port = &p
		}

		if init.Protocol == nil && init.Hostname == nil && init.Port == nil && init.Pathname == nil {
			p := processBaseURLString(baseURL.EscapedPath(), iType)
			result.Pathname = &p
		}

		if init.Protocol == nil && init.Hostname == nil && init.Port == nil && init.Pathname == nil && init.Search == nil {
			s := processBaseURLString(baseURL.RawQuery, iType)
			result.Search = &s
		}

		if init.Protocol == nil && init.Hostname == nil && init.Port == nil && init.Pathname == nil && init.Search == nil && init.Hash == nil {
			h := processBaseURLString(baseURL.EscapedFragment(), iType)
			result.Hash = &h
		}
	}

	if init.Protocol != nil {
		p, err := processProtocolForInit(*init.Protocol, iType)
		if err != nil {
			return nil, err
		}

		result.Protocol = &p
	}

	if init.Username != nil {
		u, err := processUsernameForInit(*init.Username, iType)
		if err != nil {
			return nil, err
		}

		result.Username = &u
	}

	if init.Password != nil {
		p, err := processPasswordForInit(*init.Password, iType)
		if err != nil {
			return nil, err
		}

		result.Password = &p
	}

	if init.Hostname != nil {
		h := processHostnameForInit(*init.Hostname, iType)
		result.Hostname = &h
	}

	var proto string
	if result.Protocol == nil {
		proto = ""
	} else {
		proto = *result.Protocol
	}

	if init.Port != nil {

		p := processPortForInit(*init.Port, proto, iType)
		result.Port = &p
	}

	if init.Pathname != nil {
		result.Pathname = init.Pathname

		if baseURL != nil && baseURL.Opaque != "" && !isAbsolutePathname(*result.Pathname, iType) {
			baseURLPath := processBaseURLString(baseURL.Opaque, iType)
			p := strings.TrimSuffix(baseURLPath, "/") + *result.Pathname
			result.Pathname = &p
		}

		p := processPathnameForInit(*result.Pathname, proto, iType)
		result.Pathname = &p
	}

	if init.Search != nil {
		s := processSearchForInit(*init.Search, iType)
		result.Search = &s
	}

	if init.Hash != nil {
		h := processHashForInit(*init.Hash, iType)
		result.Hash = &h
	}

	return result, nil
}

// https://urlpattern.spec.whatwg.org/#process-a-base-url-string
func processBaseURLString(input, uType string) string {
	if uType != "pattern" {
		return input
	}

	return escapePatternString(input)
}

// https://urlpattern.spec.whatwg.org/#process-protocol-for-init
func processProtocolForInit(value, pType string) (string, error) {
	strippedValue := strings.TrimSuffix(value, ":")

	if pType == "pattern" {
		return strippedValue, nil
	}

	return canonicalizeProtocol(strippedValue)
}

// https://urlpattern.spec.whatwg.org/#process-username-for-init
func processUsernameForInit(value, uType string) (string, error) {
	if uType == "pattern" {
		return value, nil
	}

	return canonicalizeUsername(value)
}

// https://urlpattern.spec.whatwg.org/#process-password-for-init
func processPasswordForInit(value, uType string) (string, error) {
	if uType == "pattern" {
		return value, nil
	}

	return canonicalizePassword(value)
}

// https://urlpattern.spec.whatwg.org/#process-hostname-for-init
func processHostnameForInit(value, uType string) string {
	if uType == "pattern" {
		return value
	}

	result, _ := canonicalizeHostname(value)

	return result
}

// https://urlpattern.spec.whatwg.org/#process-port-for-init
func processPortForInit(port, protocol, pType string) string {
	result, _ := canonicalizePort(port, protocol)

	return result
}

// https://urlpattern.spec.whatwg.org/#process-pathname-for-init
func processPathnameForInit(pathnameValue, protocolValue, ptype string) string {
	if ptype == "pattern" {
		return pathnameValue
	}

	if protocolValue == "" {
		result, _ := canonicalizePathname(pathnameValue)

		return result
	}

	for _, ss := range specialSchemeList {
		if protocolValue != ss {
			continue
		}

		result, _ := canonicalizePathname(pathnameValue)

		return result
	}

	result, _ := canonicalizeOpaquePathname(pathnameValue)

	return result
}

// https://urlpattern.spec.whatwg.org/#process-search-for-init
func processSearchForInit(value, sType string) string {
	strippedValue := strings.TrimPrefix(value, "?")

	if sType == "pattern" {
		return strippedValue
	}

	result, _ := canonicalizeSearch(strippedValue)

	return result
}

// https://urlpattern.spec.whatwg.org/#process-hash-for-init
func processHashForInit(value, hType string) string {
	strippedValue := strings.TrimPrefix(value, "#")

	if hType == "pattern" {
		return strippedValue
	}

	result, _ := canonicalizeHash(value)

	return result
}

// https://urlpattern.spec.whatwg.org/#is-an-absolute-pathname
func isAbsolutePathname(input, pType string) bool {
	if input == "" {
		return false
	}

	if input[0] == '/' {
		return true
	}

	if pType == "url" {
		return false
	}

	return strings.HasPrefix(input, "\\/") || strings.HasPrefix(input, "{/")
}

// https://urlpattern.spec.whatwg.org/#hostname-pattern-is-an-ipv6-address
func hostnamePatternIsIPv6Address(input string) bool {
	if len(input) < 2 {
		return false
	}

	i := []rune(input)

	if i[0] == '[' {
		return true
	}
	if i[0] == '{' && i[1] == '[' {
		return true
	}
	if i[0] == '\\' && i[1] == '[' {
		return true
	}

	return false
}
