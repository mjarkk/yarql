package graphql

import (
	"errors"
)

type parserCtx struct {
	res    []byte
	query  []byte
	charNr int
	errors []error
}

func (ctx *parserCtx) parseQueryToBytecode() {
	*ctx = parserCtx{
		res:    ctx.res[:0],
		query:  ctx.query,
		charNr: 0,
		errors: ctx.errors[:0],
	}

	ctx.parseOperatorOrFragment()
}

func (ctx *parserCtx) parseOperatorOrFragment() {
	c := ctx.mightIgnoreNextTokens()

	if c == '{' {

	}
}

//
// ITERATOR HELPERS
//

func (ctx *parserCtx) checkC(nr int) (res byte, end bool) {
	if ctx.eof(nr) {
		return 0, true
	}
	return ctx.c(nr), false
}

func (ctx *parserCtx) c(nr int) byte {
	return ctx.query[nr]
}

func (ctx *parserCtx) eof(nr int) bool {
	return nr >= len(ctx.query)
}

func (ctx *parserCtx) currentC() byte {
	return ctx.c(ctx.charNr)
}

func (ctx *parserCtx) mightIgnoreNextTokens() (nextC byte, eof bool) {
	for {
		c, eof := ctx.checkC(ctx.charNr)
		if eof {
			return 0, true
		}

		isIgnoredChar := ctx.isIgnoredToken(c)
		if !isIgnoredChar {
			return c, false
		}

		ctx.charNr++
	}
}

// https://spec.graphql.org/June2018/#sec-Source-Text.Ignored-Tokens
func (ctx *parserCtx) isIgnoredToken(c byte) bool {
	// TODO this doesn't support unicode bomb
	return c == ' ' || c == '\t' || ctx.isLineTerminator() || ctx.isComment(true) || c == 0
}

// https://spec.graphql.org/June2018/#LineTerminator
func (ctx *parserCtx) isLineTerminator() bool {
	c := ctx.currentC()
	if c == '\n' {
		return true
	}
	if c == '\r' {
		next, _ := ctx.checkC(ctx.charNr + 1)
		if next == '\n' {
			ctx.charNr++
		}
		return true
	}
	return false
}

// https://spec.graphql.org/June2018/#Comment
func (ctx *parserCtx) isComment(parseComment bool) bool {
	if ctx.currentC() == '#' {
		if parseComment {
			ctx.parseComment()
		}
		return true
	}
	return false
}

func (ctx *parserCtx) parseComment() {
	for {
		if ctx.eof(ctx.charNr) {
			return
		}
		if ctx.isLineTerminator() {
			return
		}
		ctx.charNr++
	}
}

func (ctx *parserCtx) matches(oneOf ...string) int {
	startIdx := ctx.charNr

	lastChecked := ""
	for {
		c, eof := ctx.checkC(ctx.charNr)
		if eof {
			ctx.charNr = startIdx
			return -1
		}
		offset := ctx.charNr - startIdx

		for idx, key := range oneOf {
			keyLen := len(key)
			if offset < keyLen {
				if key[offset] != c {
					// Nullify value so we won't check it again
					oneOf[idx] = ""
				} else if keyLen == offset+1 {
					ctx.charNr++
					return idx
				} else {
					lastChecked = key
				}
			}
		}

		if lastChecked == "" {
			ctx.charNr = startIdx
			return -1
		}

		ctx.charNr++
	}
}

type ErrorWLocation struct {
	err    error
	line   uint
	column uint
}

func (e ErrorWLocation) Error() string {
	return e.err.Error()
}

func (ctx *parserCtx) err(err string) bool {
	line := uint(1)
	column := uint(0)
	for idx, char := range ctx.query {
		if idx == ctx.charNr {
			break
		}

		switch char {
		case '\n':
			if column == 0 && idx > 0 && ctx.query[idx-1] == '\r' {
				// don't count \r\n as 2 lines
				continue
			}
			line++
			column = 0
		case '\r':
			line++
			column = 0
		default:
			column++
		}
	}

	ctx.errors = append(ctx.errors, ErrorWLocation{
		errors.New(err),
		line,
		uint(column),
	})
	return true
}

func (ctx *parserCtx) unexpectedEOF() bool {
	return ctx.err("unexpected EOF")
}

func (ctx *parserCtx) parseAndWriteName() (notEmpty bool, criticalError bool) {
	written := false
	for {
		c, eof := ctx.checkC(ctx.charNr)
		if eof {
			return written, ctx.unexpectedEOF()
		}

		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || (written && c >= '0' && c <= '9') {
			ctx.res = append(ctx.res, c)
			ctx.charNr++
			written = true
			continue
		}

		return written, false
	}
}
