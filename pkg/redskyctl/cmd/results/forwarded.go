package results

import (
	"net/http"
	"strconv"
	"strings"
	"text/scanner"
)

// Forwarded is an implementation of an RFC 7239 "Forwarded" header
type Forwarded []Forward

// Forward represents an individual hop in a forwarding chain
type Forward struct {
	By    string
	For   string
	Host  string
	Proto string
}

func (f Forwarded) FirstHostOrDefault(host string) string {
	if len(f) == 0 || f[0].Host == "" {
		return host
	}
	return f[0].Host
}

// String returns a formatted Forwarded header value
func (f Forwarded) String() string {
	var strs []string
	for _, fwd := range f {
		strs = append(strs, fwd.String())
	}
	return strings.Join(strs, ", ")
}

// String returns the string representation of an individual hop
func (f *Forward) String() string {
	var parts []string
	parts = appendParameter(parts, "for", f.For)
	parts = appendParameter(parts, "by", f.By)
	parts = appendParameter(parts, "host", f.Host)
	parts = appendParameter(parts, "proto", f.Proto)
	return strings.Join(parts, ";")
}

// set establishes a parameter value without validation
func (f *Forward) set(parameter, identifier string) {
	switch strings.ToLower(parameter) {
	case "by":
		f.By = identifier
	case "for":
		f.For = identifier
	case "host":
		f.Host = identifier
	case "proto":
		f.Proto = identifier
	}
}

// appendParameter adds a parameter/identifier pair to a list of strings
func appendParameter(parts []string, parameter, identifier string) []string {
	switch {
	case identifier == "":
		return parts
	case needsQuoting(identifier):
		return append(parts, parameter+"="+strconv.Quote(identifier))
	default:
		return append(parts, parameter+"="+identifier)
	}
}

// token checks to see if the supplied character matches the RFC 7230 section 3.2.6 definition of the token production
func token(ch rune, i int) bool {
	return (ch >= 0x21 && ch <= 0x7e) && !strings.ContainsRune(`"(),/:;<=>?@[\]{}`, ch)
}

// needsQuoting checks if the supplied string requires quoting
func needsQuoting(value string) bool {
	for i, ch := range value {
		if !token(ch, i) {
			return true
		}
	}
	return false
}

// normalizeXForwarded expands an X-Forwarded-* header by splitting all of the comma separated values
func normalizeXForwarded(lines []string) []string {
	var normalized []string
	for _, line := range lines {
		for _, part := range strings.Split(line, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				normalized = append(normalized, part)
			}
		}
	}
	return normalized
}

// ParseForwarded parses the Forwarded header from the supplied header collection; X-Forwarded-* headers are also recognized
func ParseForwarded(h http.Header) Forwarded {
	lines, ok := h["Forwarded"]
	if ok {
		var fwdd Forwarded
		for _, line := range lines {
			fwd := Forward{}
			var s scanner.Scanner
			s.Init(strings.NewReader(line))
			s.Mode = scanner.ScanIdents | scanner.ScanStrings
			s.IsIdentRune = token

			buf := make([]string, 2)
			bufPos := 0
			for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
				switch tok {
				case scanner.Ident:
					buf[bufPos] = s.TokenText()
				case scanner.String:
					buf[bufPos], _ = strconv.Unquote(s.TokenText())
				case '=':
					bufPos = 1
				case ';', ',':
					fwd.set(buf[0], buf[1])
					bufPos = 0
					buf[0] = ""
					buf[1] = ""
					if tok == ',' {
						fwdd = append(fwdd, fwd)
						fwd = Forward{}
					}
				}
			}
			if bufPos == 1 {
				fwd.set(buf[0], buf[1])
			}
			fwdd = append(fwdd, fwd)
		}
		return fwdd
	}

	// Translate X-Forwarded-* headers, RFC 7239 section 7.4
	xForwarded := Forward{
		Host:  h.Get("X-Forwarded-Host"),
		Proto: h.Get("X-Forwarded-Proto"),
	}
	forwardedBy := normalizeXForwarded(h["X-Forwarded-By"])
	forwardedFor := normalizeXForwarded(h["X-Forwarded-For"])
	if len(forwardedBy) == 1 && len(forwardedFor) < 2 {
		xForwarded.By = forwardedBy[0]
	}
	if len(forwardedFor) == 1 && len(forwardedBy) < 2 {
		xForwarded.For = forwardedFor[0]
	}
	return Forwarded{xForwarded}
}

// ApplyForwardedToOutgoingRequest updates the Forwarded header before the passing the request on
func ApplyForwardedToOutgoingRequest(req *http.Request, fwdBy string) {
	fwd := Forward{
		By:    fwdBy,
		For:   req.RemoteAddr,
		Host:  req.Host,
		Proto: req.URL.Scheme,
	}
	if fwd.Proto == "" && req.TLS != nil {
		fwd.Proto = "https"
	}
	if fwd.Proto == "" {
		fwd.Proto = "http"
	}
	fwdd := append(ParseForwarded(req.Header), fwd)
	req.Header.Set("Forwarded", fwdd.String())
}
