package urlpattern

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/dunglas/whatwg-url/canonicalizer"
	"github.com/dunglas/whatwg-url/url"
)

// https://urlpattern.spec.whatwg.org/#full-wildcard-regexp-value
const fullWildcardRegexpValue = ".*"

// Experimental: this symbol is exported to allow users adding new values, but may be removed in the feature.
// TODO: there is nothing in the Go stdlib to find the default port associated with a protocol.
// Let's just replace values for protocols in specialSchemeList for now.
// This list could be completed using https://en.wikipedia.org/wiki/List_of_TCP_and_UDP_port_numbers
var DefaultPorts = map[string]string{
	"http":  "80",
	"https": "443",
	"ws":    "80",
	"wss":   "443",
	"ftp":   "21",
}

var urlParser = url.NewParser()
var hostnameParser = canonicalizer.New(url.WithFailOnValidationError(), canonicalizer.WithDefaultScheme("http"))

var (
	NonEmptySuffixError      = errors.New("suffix must be the empty string")
	BadParserIndexError      = errors.New("parser's index must be less than parser's token list size")
	DuplicatePartNameError   = errors.New("duplicate name")
	RequiredTokenError       = errors.New("missing required token")
	InvalidIPv6HostnameError = errors.New("invalid IPv6 hostname")
	InvalidPortError         = errors.New("invalid port")
)

// https://urlpattern.spec.whatwg.org/#encoding-callback
type encodingCallback func(string) (string, error)

// https://urlpattern.spec.whatwg.org/#parse-a-pattern-string
func parsePatternString(input string, options options, encodingCallback encodingCallback) (partList, error) {
	tl, err := tokenize(input, tokenizePolicyStrict)
	if err != nil {
		return nil, err
	}

	p := patternParser{
		encodingCallback:      encodingCallback,
		segmentWildcardRegexp: generateSegmentWildcardRegexp(options),
		tokenList:             tl,
	}

	tls := len(tl)

	for p.index < tls {
		charToken, err := p.tryConsumeToken(tokenChar)
		if err != nil {
			return nil, err
		}

		nameToken, err := p.tryConsumeToken(tokenName)
		if err != nil {
			return nil, err
		}

		regexpOrWildcardToken, err := p.tryConsumeRegexpOrWildcardToken(nameToken)
		if err != nil {
			return nil, err
		}

		if nameToken != nil || regexpOrWildcardToken != nil {
			prefix := ""
			if charToken != nil {
				prefix = charToken.value
			}

			if prefix != "" && prefix != string(options.prefixCodePoint) {
				p.pendingFixedValue = p.pendingFixedValue + prefix
				prefix = ""
			}

			if err := p.maybeAddPartFromPendingFixedValue(); err != nil {
				return nil, err
			}

			modifierToken, err := p.tryConsumeModifierToken()
			if err != nil {
				return nil, err
			}
			if err := p.addPart(prefix, nameToken, regexpOrWildcardToken, "", modifierToken); err != nil {
				return nil, err
			}

			continue
		}

		fixedToken := charToken
		if fixedToken == nil {
			fixedToken, err = p.tryConsumeToken(tokenEscapedChar)
			if err != nil {
				return nil, err
			}
		}
		if fixedToken != nil {
			p.pendingFixedValue = p.pendingFixedValue + fixedToken.value

			continue
		}

		openToken, err := p.tryConsumeToken(tokenOpen)
		if err != nil {
			return nil, err
		}

		if openToken != nil {
			prefix, err := p.consumeText()
			if err != nil {
				return nil, err
			}

			nameToken, err := p.tryConsumeToken(tokenName)
			if err != nil {
				return nil, err
			}

			regexpOrWildcardToken, err := p.tryConsumeRegexpOrWildcardToken(nameToken)
			if err != nil {
				return nil, err
			}

			suffix, err := p.consumeText()
			if err != nil {
				return nil, err
			}

			if _, err := p.consumeRequiredToken(tokenClose); err != nil {
				return nil, fmt.Errorf("missing close token: %w", err)
			}

			modifierToken, err := p.tryConsumeModifierToken()
			if err != nil {
				return nil, err
			}

			if err := p.addPart(prefix, nameToken, regexpOrWildcardToken, suffix, modifierToken); err != nil {
				return nil, err
			}

			continue
		}

		if err := p.maybeAddPartFromPendingFixedValue(); err != nil {
			return nil, err
		}

		if _, err := p.consumeRequiredToken(tokenEnd); err != nil {
			return nil, fmt.Errorf("missing end token: %w", err)
		}
	}

	return p.partList, nil
}

type patternParser struct {
	tokenList             []token
	encodingCallback      encodingCallback
	segmentWildcardRegexp string
	partList              partList
	pendingFixedValue     string
	index                 int
	nextNumericName       float64
}

// https://urlpattern.spec.whatwg.org/#try-to-consume-a-token
func (p *patternParser) tryConsumeToken(tokenType tokenType) (*token, error) {
	// Assert: parser’s index is less than parser’s token list size.
	if p.index >= len(p.tokenList) {
		return nil, BadParserIndexError
	}

	nextToken := p.tokenList[p.index]
	if nextToken.tType != tokenType {
		return nil, nil
	}

	p.index++

	return &nextToken, nil
}

// https://urlpattern.spec.whatwg.org/#try-to-consume-a-regexp-or-wildcard-token
func (p *patternParser) tryConsumeRegexpOrWildcardToken(nameToken *token) (*token, error) {
	token, err := p.tryConsumeToken(tokenRegexp)
	if err != nil {
		return nil, err
	}
	if nameToken == nil && token == nil {
		token, err = p.tryConsumeToken(tokenAsterisk)
		if err != nil {
			return nil, err
		}
	}

	return token, nil
}

// https://urlpattern.spec.whatwg.org/#maybe-add-a-part-from-the-pending-fixed-value
func (p *patternParser) maybeAddPartFromPendingFixedValue() error {
	if p.pendingFixedValue == "" {
		return nil
	}

	encodedValue, err := p.encodingCallback(p.pendingFixedValue)
	if err != nil {
		return err
	}

	p.pendingFixedValue = ""

	part := part{pType: partFixedText, value: encodedValue, modifier: partModifierNone}
	p.partList = append(p.partList, part)

	return nil
}

// https://urlpattern.spec.whatwg.org/#try-to-consume-a-modifier-token
func (p *patternParser) tryConsumeModifierToken() (*token, error) {
	token, err := p.tryConsumeToken(tokenOtherModifier)
	if err != nil {
		return nil, err
	}
	if token != nil {
		return token, nil
	}

	return p.tryConsumeToken(tokenAsterisk)
}

// https://urlpattern.spec.whatwg.org/#add-a-part
func (p *patternParser) addPart(prefix string, nameToken *token, regexpOrWildcardToken *token, suffix string, modifierToken *token) error {
	modifier := partModifierNone
	if modifierToken != nil {
		switch modifierToken.value {
		case "?":
			modifier = partModifierOptional
		case "*":
			modifier = partModifierZeroOrMore
		case "+":
			modifier = partModifierOneOrMore
		}
	}

	if nameToken == nil && regexpOrWildcardToken == nil && modifier == partModifierNone {
		p.pendingFixedValue = p.pendingFixedValue + prefix

		return nil
	}

	if err := p.maybeAddPartFromPendingFixedValue(); err != nil {
		return err
	}

	if nameToken == nil && regexpOrWildcardToken == nil {
		// Assert: suffix is the empty string.
		if suffix != "" {
			return NonEmptySuffixError
		}

		if prefix == "" {
			return nil
		}

		encodedValue, err := p.encodingCallback(prefix)
		if err != nil {
			return err
		}

		part := part{pType: partFixedText, value: encodedValue, modifier: modifier}
		p.partList = append(p.partList, part)

		return nil
	}

	regexpValue := ""
	if regexpOrWildcardToken == nil {
		regexpValue = p.segmentWildcardRegexp
	} else if regexpOrWildcardToken.tType == tokenAsterisk {
		regexpValue = fullWildcardRegexpValue
	} else {
		regexpValue = regexpOrWildcardToken.value
	}

	pType := partRegexp
	switch regexpValue {
	case p.segmentWildcardRegexp:
		pType = partSegmentWildcard
		regexpValue = ""
	case fullWildcardRegexpValue:
		pType = partFullWildcard
		regexpValue = ""

	}

	name := ""
	if nameToken != nil {
		name = nameToken.value
	} else if regexpOrWildcardToken != nil {
		name = strconv.FormatFloat(p.nextNumericName, 'f', -1, 64)
		p.nextNumericName++
	}

	if p.isDuplicateName(name) {
		return DuplicatePartNameError
	}

	encodedPrefix, err := p.encodingCallback(prefix)
	if err != nil {
		return err
	}

	encodedSuffix, err := p.encodingCallback(suffix)
	if err != nil {
		return err
	}

	part := part{pType: pType, value: regexpValue, modifier: modifier, name: name, prefix: encodedPrefix, suffix: encodedSuffix}
	p.partList = append(p.partList, part)

	return nil
}

// https://urlpattern.spec.whatwg.org/#is-a-duplicate-name
func (p *patternParser) isDuplicateName(name string) bool {
	for _, part := range p.partList {
		if part.name == name {
			return true
		}
	}

	return false
}

// https://urlpattern.spec.whatwg.org/#consume-text
func (p *patternParser) consumeText() (string, error) {
	var result strings.Builder
	for {
		token, err := p.tryConsumeToken(tokenChar)
		if err != nil {
			return "", err
		}
		if token == nil {
			token, err = p.tryConsumeToken(tokenEscapedChar)
			if err != nil {
				return "", err
			}
		}
		if token == nil {
			break
		}
		result.WriteString(token.value)
	}

	return result.String(), nil
}

// https://urlpattern.spec.whatwg.org/#consume-a-required-token
func (p *patternParser) consumeRequiredToken(tokenType tokenType) (*token, error) {
	result, err := p.tryConsumeToken(tokenType)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, RequiredTokenError
	}

	return result, nil
}

// https://urlpattern.spec.whatwg.org/#generate-a-segment-wildcard-regexp
func generateSegmentWildcardRegexp(options options) string {
	return "[^" + escapeRegexpString(string(options.delimiterCodePoint)) + "]+?"
}

// https://urlpattern.spec.whatwg.org/#canonicalize-a-protocol
func canonicalizeProtocol(value string) (string, error) {
	if value == "" {
		return value, nil
	}

	dummyURL, err := urlParser.Parse(value + "://dummy.test")
	if err != nil {
		return "", err
	}

	return dummyURL.Scheme(), nil
}

// https://urlpattern.spec.whatwg.org/#canonicalize-a-username
func canonicalizeUsername(value string) (string, error) {
	if value == "" {
		return value, nil
	}

	return urlParser.PercentEncodeString(value, url.UserInfoPercentEncodeSet), nil
}

// https://urlpattern.spec.whatwg.org/#canonicalize-a-password
func canonicalizePassword(value string) (string, error) {
	if value == "" {
		return value, nil
	}

	return urlParser.PercentEncodeString(value, url.UserInfoPercentEncodeSet), nil
}

// https://urlpattern.spec.whatwg.org/#canonicalize-a-hostname
// https://github.com/whatwg/urlpattern/issues/220#issuecomment-2074613501
func canonicalizeHostname(hostnameValue, protocolValue string) (string, error) {
	if hostnameValue == "" {
		return hostnameValue, nil
	}

	// Dirty workaround for https://github.com/whatwg/urlpattern/issues/206
	if hostnameValue[:1] != "[" {
		for _, c := range hostnameValue {
			if c == '/' || c == '?' || c == '#' || c == ':' || c == '\\' {
				return "", errors.New("invalid hostname")
			}
		}
	}

	var (
		u   *url.Url
		err error
	)

	if protocolValue == "" {
		u = hostnameParser.NewUrl()
	} else {
		u, err = hostnameParser.Parse(protocolValue + "://dummy.test")
		if err != nil {
			return "", err
		}
	}

	u, err = hostnameParser.BasicParser(hostnameValue, nil, u, url.StateHostname)
	if err != nil {
		return "", err
	}

	return u.Hostname(), nil
}

// https://github.com/whatwg/urlpattern/issues/220#issuecomment-2074613501
func canonicalizeDomainName(value string) (string, error) {
	return canonicalizeHostname(value, "https")
}

// https://urlpattern.spec.whatwg.org/#canonicalize-a-port
func canonicalizePort(portValue, protocolValue string) (string, error) {
	if portValue == "" {
		return portValue, nil
	}

	var (
		u   *url.Url
		err error
	)

	if protocolValue == "" {
		u = hostnameParser.NewUrl()
	} else {
		u, err = hostnameParser.Parse(protocolValue + "://dummy.test")
		if err != nil {
			return "", err
		}
	}

	u, err = hostnameParser.BasicParser(portValue, nil, u, url.StatePort)
	if err != nil {
		return "", err
	}

	p := u.Port()

	// This looks like a bug in the spec ("80 " should be considered valid), but there is a test covering this
	// Another dirty workaround
	if p != portValue {
		if dp, ok := DefaultPorts[protocolValue]; ok && portValue == dp {
			return p, nil
		}

		return "", InvalidPortError
	}

	return p, nil
}

// https://urlpattern.spec.whatwg.org/#canonicalize-a-pathname
// TODO: Note, implementations are free to simply disable slash prepending in their URL parsing code instead of paying the performance penalty of inserting and removing characters in this algorithm.
func canonicalizePathname(value string) (string, error) {
	if value == "" {
		return value, nil
	}

	leadingSlash := []rune(value)[0] == '/'
	var modifiedValue strings.Builder

	if !leadingSlash {
		modifiedValue.WriteString("/-")
	}

	modifiedValue.WriteString(value)

	dummyURL := urlParser.NewUrl()
	u, err := urlParser.BasicParser(modifiedValue.String(), nil, dummyURL, url.StatePathStart)
	if err != nil {
		return "", err
	}

	result := u.Pathname()

	if !leadingSlash {
		result = result[2:]
	}

	return result, nil
}

// https://urlpattern.spec.whatwg.org/#canonicalize-an-opaque-pathname
func canonicalizeOpaquePathname(value string) (string, error) {
	if value == "" {
		return value, nil
	}

	var err error
	dummyURL := urlParser.NewUrl()

	u, err := urlParser.BasicParser(value, nil, dummyURL, url.StateOpaquePath)
	if err != nil {
		return "", err
	}

	return u.Pathname(), nil
}

// https://urlpattern.spec.whatwg.org/#canonicalize-a-search
func canonicalizeSearch(value string) (string, error) {
	if value == "" {
		return value, nil
	}

	dummyURL := urlParser.NewUrl()

	u, err := urlParser.BasicParser(value, nil, dummyURL, url.StateQuery)
	if err != nil {
		return "", err
	}

	return u.Query(), nil
}

// https://urlpattern.spec.whatwg.org/#canonicalize-a-hash
func canonicalizeHash(value string) (string, error) {
	if value == "" {
		return value, nil
	}

	dummyURL := urlParser.NewUrl()
	u, err := urlParser.BasicParser(value, nil, dummyURL, url.StateFragment)
	if err != nil {
		return "", nil
	}

	return u.Fragment(), nil
}

// https://urlpattern.spec.whatwg.org/#canonicalize-an-ipv6-hostname
func canonicalizeIPv6Hostname(value string) (string, error) {
	var result strings.Builder

	for _, c := range value {
		if c != '[' && c != ']' && c != ':' && !unicode.Is(unicode.ASCII_Hex_Digit, c) {
			return "", InvalidIPv6HostnameError
		}

		result.WriteRune(unicode.ToLower(c))
	}

	return result.String(), nil
}
