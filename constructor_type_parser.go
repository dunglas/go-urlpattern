package urlpattern

import "golang.org/x/exp/utf8string"

// https://wicg.github.io/urlpattern/#constructor-string-parsing

type constructorTypeParser struct {
	input                         utf8string.String
	tokenList                     []token
	result                        urlPatternInit
	componentStart                int
	tokenIndex                    int
	tokenIncrement                int
	groupDepth                    int
	hostnameIPv6BracketDepth      int
	protocolMatchesASpecialScheme bool
	state                         state
}

// https://wicg.github.io/urlpattern/#constructor-string-parser-state
type state uint8

const (
	sInit state = iota
	protocol
	authority
	username
	password
	hostname
	port
	pathname
	search
	hash
	done
)

func newConstructorTypeParser(input string, tokenList []token) constructorTypeParser {
	return constructorTypeParser{
		input:          *utf8string.NewString(input),
		tokenList:      tokenList,
		result:         urlPatternInit{},
		tokenIncrement: 1,
		state:          sInit,
	}
}

// https://wicg.github.io/urlpattern/#constructor-string-parsing
func parseConstructorString(input string) error {
	tl, err := tokenize(input, tokenizePolicyLenient)
	if err != nil {
		return err
	}

	p := newConstructorTypeParser(input, tl)

	tlLen := len(p.tokenList)

	for p.tokenIndex < tlLen {
		p.tokenIncrement = 1

		if p.tokenList[p.tokenIndex].tType == tokenEnd {
			if p.state == sInit {
				p.rewind()

				if p.isHashPrefix() {
					p.changeState(hash, 1)
				} else if p.isSearchPrefix() {
					p.changeState(search, 1)
					//p.result.Hash = ""
				} else {
					p.changeState(pathname, 0)
					//p.result.Hash = ""
					//p.result.Search = ""
				}

				p.tokenIndex += p.tokenIncrement

				continue
			}

			if p.state == authority {
				p.rewindAndSetState(hostname)
				p.tokenIndex += p.tokenIncrement

				continue
			}

			p.changeState(done, 0)

			break
		}

		if p.isGroupOpen() {
			p.groupDepth++
			p.tokenIndex += p.tokenIncrement

			continue
		}

		if p.groupDepth > 0 {
			if p.isGroupClose() {
				p.groupDepth--
			} else {
				p.tokenIndex += p.tokenIncrement

				continue
			}
		}

		// Switch on parser’s state and run the associated steps:
	}

	return nil
}

func (p *constructorTypeParser) rewind() {
	p.tokenIndex = p.componentStart
	p.tokenIncrement = 0
}

func (p *constructorTypeParser) rewindAndSetState(s state) {
	p.rewind()
	p.state = s
}

func (p *constructorTypeParser) isHashPrefix() bool {
	return p.isNonSpecialPatternChar(p.tokenIndex, "#")
}

func (p *constructorTypeParser) isSearchPrefix() bool {
	if p.isNonSpecialPatternChar(p.tokenIndex, "?") {
		return true
	}

	if p.tokenList[p.tokenIndex].value != "?" {
		return false
	}

	previousIndex := p.tokenIndex - 1
	if previousIndex < 0 {
		return true
	}

	previousToken := p.getSafeToken(previousIndex)
	switch previousToken.tType {
	case tokenName:
		return false

	case tokenRegexp:
		return false

	case tokenClose:
		return false

	case tokenAsterisk:
		return false
	}

	return true
}

func (p *constructorTypeParser) isGroupOpen() bool {
	return p.tokenList[p.tokenIndex].tType == tokenOpen
}

func (p *constructorTypeParser) isGroupClose() bool {
	return p.tokenList[p.tokenIndex].tType == tokenClose
}

func (p *constructorTypeParser) isNonSpecialPatternChar(index int, value string) bool {
	token := p.getSafeToken(index)
	if token.value != value {
		return false
	}

	return token.tType == tokenChar || token.tType == tokenEscapedChar || token.tType == tokenInvalidChar
}

func (p *constructorTypeParser) getSafeToken(index int) token {
	len := len(p.tokenList)

	if index < len {
		return p.tokenList[index]
	}

	return p.tokenList[len-1]
}

func (p *constructorTypeParser) changeState(newState state, skip int) {
	// ignore sInit, authority and done
	switch p.state {
	case protocol:
		p.result.Protocol = p.makeComponentString()
	case username:
		p.result.Username = p.makeComponentString()
	case password:
		p.result.Password = p.makeComponentString()
	case hostname:
		p.result.Hostname = p.makeComponentString()
	case port:
		p.result.Port = p.makeComponentString()
	case pathname:
		p.result.Pathname = p.makeComponentString()
	case search:
		p.result.Search = p.makeComponentString()
	case hash:
		p.result.Hash = p.makeComponentString()
	}

	p.state = newState
	p.tokenIndex = p.tokenIndex + skip
	p.componentStart = p.tokenIndex
	p.tokenIncrement = 0
}

func (p *constructorTypeParser) makeComponentString() string {
	token := p.tokenList[p.tokenIndex]
	componentStartToken := p.getSafeToken(int(p.componentStart))
	componentStartInputIndex := componentStartToken.index
	endIndex := token.index

	return p.input.Slice(componentStartInputIndex, endIndex)
}
