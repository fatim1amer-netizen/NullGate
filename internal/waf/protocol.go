package waf

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// ProtocolGuard enforces HTTP protocol hygiene before any deeper processing.
// Its job is to reject requests that are malformed, oversized, or structured
// to smuggle past intermediaries — the substrate on which the rest of the WAF
// relies. Anything suspicious here is rejected outright (fail-closed).
type ProtocolGuard struct {
	cfg ProtocolConfig
}

// NewProtocolGuard builds a guard from the protocol section of config.
func NewProtocolGuard(cfg ProtocolConfig) *ProtocolGuard {
	return &ProtocolGuard{cfg: cfg}
}

// ProtocolError describes a protocol-level rejection with a reason code.
type ProtocolError struct {
	Reason string
	Status int
}

func (e *ProtocolError) Error() string { return e.Reason }

func protoErr(reason string, status int) *ProtocolError {
	return &ProtocolError{Reason: reason, Status: status}
}

// Check validates the structural envelope of a request. It returns nil when
// the request is acceptable, or a *ProtocolError describing the violation.
func (g *ProtocolGuard) Check(r *http.Request) error {
	if g.cfg.MaxMethodLength > 0 && len(r.Method) > g.cfg.MaxMethodLength {
		return protoErr("method too long", http.StatusBadRequest)
	}
	if len(g.cfg.AllowedMethods) > 0 {
		ok := false
		for _, m := range g.cfg.AllowedMethods {
			if r.Method == m {
				ok = true
				break
			}
		}
		if !ok {
			return protoErr("method not allowed: "+r.Method, http.StatusMethodNotAllowed)
		}
	}

	if g.cfg.MaxURLLength > 0 && len(r.URL.RequestURI()) > g.cfg.MaxURLLength {
		return protoErr("request URI too long", http.StatusRequestURITooLong)
	}
	if g.cfg.MaxQueryParams > 0 && len(r.URL.Query()) > g.cfg.MaxQueryParams {
		return protoErr("too many query parameters", http.StatusBadRequest)
	}
	if g.cfg.MaxHeaderCount > 0 && len(r.Header) > g.cfg.MaxHeaderCount {
		return protoErr("too many headers", http.StatusBadRequest)
	}

	if g.cfg.StrictVersionCheck {
		switch r.ProtoMajor {
		case 1, 2:
			// supported
		default:
			return protoErr("unsupported HTTP version: "+r.Proto, http.StatusHTTPVersionNotSupported)
		}
	}

	// Conflicting Content-Length vs Transfer-Encoding is the canonical
	// request-smuggling primitive; refuse to guess.
	if g.cfg.RejectConflictingEncoding {
		if r.TransferEncoding != nil && r.ContentLength >= 0 && r.ContentLength > 0 {
			return protoErr("conflicting content-length and transfer-encoding", http.StatusBadRequest)
		}
	}
	if g.cfg.RejectChunkedBodies && len(r.TransferEncoding) > 0 {
		return protoErr("chunked transfer encoding rejected", http.StatusBadRequest)
	}

	if g.cfg.BlockNullBytes {
		uri := r.URL.RequestURI()
		if strings.ContainsRune(uri, '\x00') {
			return protoErr("null byte in request", http.StatusBadRequest)
		}
		for k, v := range r.Header {
			if strings.ContainsRune(k, '\x00') {
				return protoErr("null byte in header name", http.StatusBadRequest)
			}
			for _, val := range v {
				if strings.ContainsRune(val, '\x00') {
					return protoErr("null byte in header value", http.StatusBadRequest)
				}
			}
		}
	}

	if g.cfg.BlockControlChars {
		uri := r.URL.RequestURI()
		for i := 0; i < len(uri); i++ {
			c := uri[i]
			if c < 0x20 && c != '\t' { // control chars other than tab
				return protoErr("control character in request URI", http.StatusBadRequest)
			}
		}
	}

	if g.cfg.EnforceContentType {
		if r.ContentLength > 0 || len(r.TransferEncoding) > 0 {
			ct := r.Header.Get("Content-Type")
			if ct == "" {
				return protoErr("missing content-type on body request", http.StatusUnsupportedMediaType)
			}
			if !g.contentTypeAllowed(ct) {
				return protoErr("content-type not allowed: "+ct, http.StatusUnsupportedMediaType)
			}
		}
	}

	// Content-Length must not claim more than we are willing to read.
	if r.ContentLength > int64(g.cfg.MaxRequestSize) {
		return protoErr("content-length exceeds maximum", http.StatusRequestEntityTooLarge)
	}

	return nil
}

func (g *ProtocolGuard) contentTypeAllowed(ct string) bool {
	mediaType := strings.ToLower(strings.TrimSpace(strings.SplitN(ct, ";", 2)[0]))
	for _, allowed := range g.cfg.MaxAllowedContentTypes {
		if mediaType == allowed {
			return true
		}
	}
	return false
}

// ValidateContentLengthMismatch compares the parsed body length against the
// declared Content-Length header, rejecting mismatches that smuggling tools
// rely on. It must be called after the body has been buffered.
func (g *ProtocolGuard) ValidateContentLengthMismatch(r *http.Request, actual int64) error {
	if !g.cfg.RejectContentLengthMismatch {
		return nil
	}
	if r.ContentLength < 0 {
		return nil // chunked or unknown; handled elsewhere
	}
	if cl, err := strconv.ParseInt(r.Header.Get("Content-Length"), 10, 64); err == nil && cl != actual {
		return protoErr(fmt.Sprintf("content-length mismatch: declared %d, actual %d", cl, actual), http.StatusBadRequest)
	}
	return nil
}
