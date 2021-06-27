package graphql

import (
	"unicode/utf8"
)

// Modified copy of https://golang.org/src/encoding/json/encode.go > encodeState.stringBytes(..)
// Copyright for function below:
//
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// IMPORTANT the full license can be found in this repo: https://github.com/golang/go
func stringToJson(s []byte, e *[]byte) {
	const hex = "0123456789abcdef"

	*e = append(*e, '"')
	start := 0
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			if b >= ' ' && b <= '}' && b != '\\' && b != '"' {
				i++
				continue
			}

			if b == '\u007f' {
				i++
				continue
			}

			if start < i {
				*e = append(*e, s[start:i]...)
			}
			*e = append(*e, '\\')
			switch b {
			case '\\', '"':
				*e = append(*e, b)
			case '\n':
				*e = append(*e, 'n')
			case '\r':
				*e = append(*e, 'r')
			case '\t':
				*e = append(*e, 't')
			default:
				// This encodes bytes < 0x20 except for \t, \n and \r.
				// If escapeHTML is set, it also escapes <, >, and &
				// because they can lead to security holes when
				// user-controlled strings are rendered into JSON
				// and served to some browsers.
				*e = append(*e, []byte(`u00`)...)
				*e = append(*e, hex[b>>4])
				*e = append(*e, hex[b&0xF])
			}
			i++
			start = i
			continue
		}
		c, size := utf8.DecodeRune(s[i:])
		if c == utf8.RuneError && size == 1 {
			if start < i {
				*e = append(*e, s[start:i]...)
			}
			*e = append(*e, []byte(`\ufffd`)...)
			i += size
			start = i
			continue
		}
		// U+2028 is LINE SEPARATOR.
		// U+2029 is PARAGRAPH SEPARATOR.
		// They are both technically valid characters in JSON strings,
		// but don't work in JSONP, which has to be evaluated as JavaScript,
		// and can lead to security holes there. It is valid JSON to
		// escape them, so we do so unconditionally.
		// See http://timelessrepo.com/json-isnt-a-javascript-subset for discussion.
		if c == '\u2028' || c == '\u2029' {
			if start < i {
				*e = append(*e, s[start:i]...)
			}
			*e = append(*e, []byte(`\u202`)...)
			*e = append(*e, hex[c&0xF])
			i += size
			start = i
			continue
		}
		i += size
	}
	if start < len(s) {
		*e = append(*e, s[start:]...)
	}
	*e = append(*e, '"')
}
