# Go URL Pattern

A spec-compliant implementation of [the WHATWG URL Pattern Living Standard](https://urlpattern.spec.whatwg.org/)
written in [Go](https://go.dev).

Tested with [web-platform-test](https://web-platform-tests.org) test suite.

## Limitations

* Some [advanced unicode features (JavaScript's `v` mode)](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/RegExp/unicodeSets) are not supported, because they are not supported by Go regular expressions.

## Credits

Created by [Kévin Dunglas](https://dunglas.fr).

Sponsored by:

* [Mercure.rocks](https://mercure.rocks)
* [Les-Tilleuls.coop](https://les-tilleuls.coop)
