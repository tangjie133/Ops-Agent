package rag

import (
	"strings"
	"unicode"
)

func tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	var b strings.Builder
	flush := func() {
		if b.Len() >= 2 {
			tokens = append(tokens, b.String())
		}
		b.Reset()
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			b.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return tokens
}

func uniqueTokens(text string) map[string]struct{} {
	toks := tokenize(text)
	out := make(map[string]struct{}, len(toks))
	for _, t := range toks {
		out[t] = struct{}{}
	}
	return out
}
