// Package canonicaljson reproduces, byte for byte, the canonical JSON the
// Decionis control plane hashes and signs for Decision Dossier artifacts:
// recursively sort object keys, leave arrays and scalars in place, then
// serialize with ECMAScript JSON.stringify semantics (the upstream reference
// is @decionis/verify's stableJsonStringify).
//
// Faithfulness notes, since Go's encoding/json differs from JSON.stringify in
// exactly the ways that would break signatures:
//   - Strings: \b and \f use their short escapes, U+2028/U+2029 are emitted
//     raw, and <, >, & are never HTML-escaped. Control characters use
//     lowercase \u00xx. Non-ASCII is emitted as raw UTF-8.
//   - Numbers: input must come from JSON produced by the control plane (an
//     ECMAScript serializer), and values must be decoded with
//     json.Decoder.UseNumber so the wire literal is preserved verbatim.
//     ES-produced JSON never contains non-canonical literals like -0 or 1.0,
//     so preserving the literal is exact.
//   - Lone UTF-16 surrogates are not reproduced (Go replaces them at decode
//     time); the control plane's serializer does not emit them.
package canonicaljson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// Decode parses raw JSON preserving number literals, producing the only value
// shapes Stringify accepts.
func Decode(raw []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("canonicaljson: decode: %w", err)
	}
	if decoder.More() {
		return nil, fmt.Errorf("canonicaljson: trailing data after JSON value")
	}
	return value, nil
}

// Stringify renders a decoded JSON value (from Decode: map[string]any, []any,
// string, json.Number, bool, nil) as canonical JSON. Any other type is an
// error rather than a guess — canonical bytes must never be approximate.
func Stringify(value any) (string, error) {
	var builder strings.Builder
	if err := write(&builder, value); err != nil {
		return "", err
	}
	return builder.String(), nil
}

func write(builder *strings.Builder, value any) error {
	switch typed := value.(type) {
	case nil:
		builder.WriteString("null")
	case bool:
		if typed {
			builder.WriteString("true")
		} else {
			builder.WriteString("false")
		}
	case json.Number:
		builder.WriteString(typed.String())
	case string:
		writeString(builder, typed)
	case []any:
		builder.WriteByte('[')
		for i, item := range typed {
			if i > 0 {
				builder.WriteByte(',')
			}
			if err := write(builder, item); err != nil {
				return err
			}
		}
		builder.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		builder.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				builder.WriteByte(',')
			}
			writeString(builder, key)
			builder.WriteByte(':')
			if err := write(builder, typed[key]); err != nil {
				return err
			}
		}
		builder.WriteByte('}')
	default:
		return fmt.Errorf("canonicaljson: unsupported type %T (decode with canonicaljson.Decode)", value)
	}
	return nil
}

const hexDigits = "0123456789abcdef"

// writeString implements ECMAScript QuoteJSONString for well-formed input.
func writeString(builder *strings.Builder, value string) {
	builder.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"':
			builder.WriteString(`\"`)
		case '\\':
			builder.WriteString(`\\`)
		case '\b':
			builder.WriteString(`\b`)
		case '\f':
			builder.WriteString(`\f`)
		case '\n':
			builder.WriteString(`\n`)
		case '\r':
			builder.WriteString(`\r`)
		case '\t':
			builder.WriteString(`\t`)
		default:
			if r < 0x20 {
				builder.WriteString(`\u00`)
				builder.WriteByte(hexDigits[(r>>4)&0xf])
				builder.WriteByte(hexDigits[r&0xf])
			} else {
				var buf [utf8.UTFMax]byte
				n := utf8.EncodeRune(buf[:], r)
				builder.Write(buf[:n])
			}
		}
	}
	builder.WriteByte('"')
}
