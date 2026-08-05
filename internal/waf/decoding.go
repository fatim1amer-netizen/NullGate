package waf

import (
	"encoding/base64"
	"encoding/hex"
	"html"
	"net/url"
	"regexp"
	"strings"
)

// Decoder normalizes potentially obfuscated input into a canonical form so
// that a single rule set can catch payloads regardless of the encoding an
// attacker layered on top. WAF bypasses overwhelmingly rely on encoding
// tricks, so aggressive-but-bounded normalization is the highest-leverage
// defense we have.
type Decoder struct {
	cfg DecodingConfig
}

// NewDecoder builds a decoder from the decoding section of the config.
func NewDecoder(cfg DecodingConfig) *Decoder {
	return &Decoder{cfg: cfg}
}

var (
	// sqlCommentRe strips SQL inline comments (/*...*/) used to break up keywords.
	sqlCommentRe = regexp.MustCompile(`/\*.*?\*/`)
	// multiSpaceRe collapses runs of whitespace (incl. tabs/newlines) to a single space.
	multiSpaceRe = regexp.MustCompile(`[\t\r\n\f\v ]+`)
	// unicodeEscRe matches \uXXXX and \xXX JavaScript/JSON style escapes.
	unicodeEscRe = regexp.MustCompile(`\\u([0-9a-fA-F]{4})|\\x([0-9a-fA-F]{2})`)
	// htmlNumEntityRe matches numeric HTML entities like &#60; and &#x3c;.
	htmlNumEntityRe = regexp.MustCompile(`&#x?[0-9a-fA-F]+;?`)
	// looksBase64Re gates base64 decoding to avoid mangling ordinary text.
	looksBase64Re = regexp.MustCompile(`^[A-Za-z0-9+/]{8,}={0,2}$`)
	// looksHexRe gates hex decoding similarly.
	looksHexRe = regexp.MustCompile(`^(0x)?[0-9a-fA-F]{8,}$`)
)

// DeepDecode repeatedly applies every enabled decoding layer until the string
// stops changing or the configured depth is reached. It returns the fully
// normalized string that the rule engine should scan.
//
// The loop is bounded by MaxDecodeDepth to prevent a crafted input from
// forcing unbounded CPU work (a classic algorithmic-complexity DoS).
func (d *Decoder) DeepDecode(payload string) string {
	current := payload
	depth := d.cfg.MaxDecodeDepth
	if depth < 1 {
		depth = 1
	}

	for i := 0; i < depth; i++ {
		next := d.decodeOnce(current)
		if next == current {
			break
		}
		current = next
	}
	return current
}

// Variants returns the set of normalized forms worth scanning. Some evasions
// only reveal themselves after aggressive normalization, but that same
// normalization can also hide a payload that was malicious in its raw form.
// Scanning both the raw and the fully-decoded string closes that gap.
func (d *Decoder) Variants(payload string) []string {
	decoded := d.DeepDecode(payload)
	if decoded == payload {
		return []string{payload}
	}
	return []string{payload, decoded}
}

// decodeOnce applies a single pass of all enabled transforms.
func (d *Decoder) decodeOnce(s string) string {
	out := s

	// 1. URL percent-decoding (handles %41, and +-as-space via QueryUnescape).
	if u, err := url.QueryUnescape(out); err == nil {
		out = u
	}

	// 2. HTML entity decoding (named + numeric), e.g. &lt; &#60; &#x3c;.
	out = html.UnescapeString(out)
	out = decodeNumericHTMLEntities(out)

	// 3. Unicode / hex escape sequences (<, \x3c).
	if d.cfg.EnableUnicodeScan {
		out = decodeUnicodeEscapes(out)
	}

	// 4. Base64 — only when the whole token plausibly is base64, to avoid
	//    corrupting ordinary text into false positives.
	if d.cfg.EnableBase64Scan {
		out = tryBase64(out)
	}

	// 5. Hex blobs (0x.... or bare hex) commonly used to smuggle SQL/JS.
	if d.cfg.EnableHexScan {
		out = tryHex(out)
	}

	// 6. Strip SQL inline comments used to split keywords (un/**/ion).
	if d.cfg.EnableCommentScan {
		out = sqlCommentRe.ReplaceAllString(out, "")
	}

	// 7. Collapse whitespace so "union    select" == "union select" and
	//    tab/newline splitting can't defeat the keyword regexes.
	if d.cfg.EnableWhitespaceScan {
		out = multiSpaceRe.ReplaceAllString(out, " ")
	}

	return out
}

// decodeNumericHTMLEntities resolves &#DDD; and &#xHH; entities that the
// stdlib html.UnescapeString may leave untouched when malformed (no semicolon).
func decodeNumericHTMLEntities(s string) string {
	return htmlNumEntityRe.ReplaceAllStringFunc(s, func(m string) string {
		body := strings.TrimPrefix(strings.TrimSuffix(m, ";"), "&#")
		var code int64
		var err error
		if len(body) > 0 && (body[0] == 'x' || body[0] == 'X') {
			code, err = parseHexInt(body[1:])
		} else {
			code, err = parseDecInt(body)
		}
		if err != nil || code <= 0 || code > 0x10FFFF {
			return m
		}
		return string(rune(code))
	})
}

// decodeUnicodeEscapes turns \uXXXX and \xXX into their runes.
func decodeUnicodeEscapes(s string) string {
	return unicodeEscRe.ReplaceAllStringFunc(s, func(m string) string {
		var hexPart string
		if strings.HasPrefix(m, `\u`) {
			hexPart = m[2:]
		} else if strings.HasPrefix(m, `\x`) {
			hexPart = m[2:]
		} else {
			return m
		}
		code, err := parseHexInt(hexPart)
		if err != nil || code < 0 || code > 0x10FFFF {
			return m
		}
		return string(rune(code))
	})
}

// tryBase64 decodes a token if it looks like base64 and the result is
// printable-ish; otherwise it returns the input unchanged.
func tryBase64(s string) string {
	trimmed := strings.TrimSpace(s)
	if !looksBase64Re.MatchString(trimmed) {
		return s
	}
	// Pad to a multiple of 4 if needed.
	if pad := len(trimmed) % 4; pad != 0 {
		trimmed += strings.Repeat("=", 4-pad)
	}
	b, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return s
	}
	decoded := string(b)
	if !mostlyPrintable(decoded) {
		return s
	}
	return decoded
}

// tryHex decodes a bare or 0x-prefixed hex blob to its byte string.
func tryHex(s string) string {
	trimmed := strings.TrimSpace(s)
	if !looksHexRe.MatchString(trimmed) {
		return s
	}
	trimmed = strings.TrimPrefix(strings.TrimPrefix(trimmed, "0x"), "0X")
	if len(trimmed)%2 != 0 {
		return s
	}
	b, err := hex.DecodeString(trimmed)
	if err != nil {
		return s
	}
	decoded := string(b)
	if !mostlyPrintable(decoded) {
		return s
	}
	return decoded
}

// mostlyPrintable reports whether a decoded blob is plausibly text (so we
// don't replace a token with binary garbage that inflates false positives).
func mostlyPrintable(s string) bool {
	if s == "" {
		return false
	}
	printable := 0
	for _, r := range s {
		if r == '\t' || r == '\n' || r == '\r' || (r >= 0x20 && r < 0x7f) || r > 0x7f {
			printable++
		}
	}
	return float64(printable)/float64(len([]rune(s))) >= 0.85
}

// parseHexInt / parseDecInt are tiny error-returning parsers used to avoid
// pulling strconv into the hot decode path with panics on bad input.
func parseHexInt(s string) (int64, error) {
	var n int64
	for _, c := range strings.ToLower(s) {
		var v int64
		switch {
		case c >= '0' && c <= '9':
			v = int64(c - '0')
		case c >= 'a' && c <= 'f':
			v = int64(c-'a') + 10
		default:
			return 0, errBadDigit
		}
		n = n*16 + v
	}
	return n, nil
}

func parseDecInt(s string) (int64, error) {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errBadDigit
		}
		n = n*10 + int64(c-'0')
	}
	return n, nil
}

// errBadDigit is a sentinel returned by the tiny numeric parsers above.
var errBadDigit = &decodeError{"bad digit"}

type decodeError struct{ msg string }

func (e *decodeError) Error() string { return e.msg }
