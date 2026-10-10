package genesismesh

// Strict JSON input (v1.2.0): refuse what parsers read differently.
//
// The canonical form of a signed record is computed from parsed JSON, so every
// implementation must parse a record to the same value. Some JSON does not
// parse alike: encoding/json keeps the last of two duplicate keys where .NET
// keeps both, and replaces a lone surrogate with U+FFFD where other parsers
// keep or refuse it. Such input is refused here, as in every implementation,
// by a named reason (the conformance suite "canonical").

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"
)

// StrictJSONError is JSON refused as input to a signed record. Reason is one
// of invalid_json (not JSON, including NaN and Infinity), duplicate_key,
// non_finite_number (1e400), integer_out_of_range (outside -2**63 ..
// 2**64 - 1), negative_zero (the integer -0) or lone_surrogate.
type StrictJSONError struct {
	Reason string
	Detail string
}

func (e *StrictJSONError) Error() string {
	return fmt.Sprintf("genesismesh: JSON refused (%s): %s", e.Reason, e.Detail)
}

var (
	minInteger = new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 63))
	maxInteger = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 64), big.NewInt(1))
)

// MaxStrictDepth is how deep arrays and objects may nest; deeper is refused
// as invalid_json, as in every implementation.
const MaxStrictDepth = 64

type strictScanner struct {
	text  []byte
	at    int
	depth int
}

func refuse(reason, detail string) error { return &StrictJSONError{Reason: reason, Detail: detail} }

func (s *strictScanner) peek() byte {
	if s.at < len(s.text) {
		return s.text[s.at]
	}
	return 0
}

func (s *strictScanner) space() {
	for s.at < len(s.text) {
		switch s.text[s.at] {
		case ' ', '\t', '\n', '\r':
			s.at++
		default:
			return
		}
	}
}

// str reads a string at the quote; with keep it returns the decoded text.
func (s *strictScanner) str(keep bool) (string, error) {
	s.at++
	var units []uint16
	pending := -1
	for {
		if s.at >= len(s.text) {
			return "", refuse("invalid_json", "a string is not closed")
		}
		b := s.text[s.at]
		var unit []uint16
		switch {
		case b == '"':
			s.at++
			if pending >= 0 {
				return "", refuse("lone_surrogate", "a high surrogate without its low half")
			}
			if !keep {
				return "", nil
			}
			return string(utf16Decode(units)), nil
		case b < 0x20:
			return "", refuse("invalid_json", "a control character in a string")
		case b == '\\':
			if s.at+1 >= len(s.text) {
				return "", refuse("invalid_json", "an unknown escape")
			}
			e := s.text[s.at+1]
			s.at += 2
			switch e {
			case 'u':
				if s.at+4 > len(s.text) {
					return "", refuse("invalid_json", "a malformed \\u escape")
				}
				v, err := strconv.ParseUint(string(s.text[s.at:s.at+4]), 16, 16)
				if err != nil {
					return "", refuse("invalid_json", "a malformed \\u escape")
				}
				s.at += 4
				unit = []uint16{uint16(v)}
			case '"', '\\', '/':
				unit = []uint16{uint16(e)}
			case 'b':
				unit = []uint16{8}
			case 'f':
				unit = []uint16{12}
			case 'n':
				unit = []uint16{10}
			case 'r':
				unit = []uint16{13}
			case 't':
				unit = []uint16{9}
			default:
				return "", refuse("invalid_json", "an unknown escape")
			}
		default:
			r, size := utf8.DecodeRune(s.text[s.at:])
			if r == utf8.RuneError && size <= 1 {
				return "", refuse("invalid_json", "text that is not UTF-8")
			}
			s.at += size
			if r >= 0x10000 {
				r -= 0x10000
				unit = []uint16{uint16(0xd800 + (r >> 10)), uint16(0xdc00 + (r & 0x3ff))}
			} else {
				unit = []uint16{uint16(r)}
			}
		}
		for _, u := range unit {
			switch {
			case pending >= 0:
				if u < 0xdc00 || u > 0xdfff {
					return "", refuse("lone_surrogate", "a high surrogate without its low half")
				}
				if keep {
					units = append(units, uint16(pending), u)
				}
				pending = -1
			case u >= 0xd800 && u <= 0xdbff:
				pending = int(u)
			case u >= 0xdc00 && u <= 0xdfff:
				return "", refuse("lone_surrogate", "a low surrogate without its high half")
			case keep:
				units = append(units, u)
			}
		}
	}
}

func utf16Decode(units []uint16) []rune {
	out := make([]rune, 0, len(units))
	for i := 0; i < len(units); i++ {
		u := rune(units[i])
		if u >= 0xd800 && u <= 0xdbff && i+1 < len(units) {
			out = append(out, ((u-0xd800)<<10)+(rune(units[i+1])-0xdc00)+0x10000)
			i++
			continue
		}
		out = append(out, u)
	}
	return out
}

// maxIntegerDigits is the length of 2**64 - 1: a longer integer is out of
// range whatever its digits (v1.3.1).
const maxIntegerDigits = 20

// number reads a number by the RFC 8259 grammar: a "." or an exponent letter
// commits to a fraction or an exponent, which must then have its digits
// ("-0.", "1e" and "1E+" are invalid_json). Only a whole number goes on to
// the negative_zero, range and precision checks; what follows it is the next
// token ("[-01]" is negative_zero for the "-0" before the "1").
func (s *strictScanner) number() error {
	start := s.at
	digits := func() bool {
		from := s.at
		for s.at < len(s.text) && s.text[s.at] >= '0' && s.text[s.at] <= '9' {
			s.at++
		}
		return s.at > from
	}
	if s.peek() == '-' {
		s.at++
	}
	switch c := s.peek(); {
	case c == '0':
		s.at++
	case c >= '1' && c <= '9':
		digits()
	default:
		return refuse("invalid_json", "a malformed number")
	}
	integer := true
	if s.peek() == '.' {
		s.at++
		integer = false
		if !digits() {
			return refuse("invalid_json", "a malformed number")
		}
	}
	if c := s.peek(); c == 'e' || c == 'E' {
		s.at++
		integer = false
		if c := s.peek(); c == '+' || c == '-' {
			s.at++
		}
		if !digits() {
			return refuse("invalid_json", "a malformed number")
		}
	}
	literal := string(s.text[start:s.at])
	if integer {
		if literal == "-0" {
			return refuse("negative_zero", "the integer -0")
		}
		// v1.3.1: refused before big.Int, whose parse time grows with the
		// square of the length (3M digits took seconds).
		if len(strings.TrimPrefix(literal, "-")) > maxIntegerDigits {
			return refuse("integer_out_of_range", fmt.Sprintf("an integer longer than %d digits", maxIntegerDigits))
		}
		n, _ := new(big.Int).SetString(literal, 10)
		if n.Cmp(minInteger) < 0 || n.Cmp(maxInteger) > 0 {
			return refuse("integer_out_of_range", literal+" is outside the 64-bit range")
		}
		return nil
	}
	if _, err := strconv.ParseFloat(literal, 64); err != nil {
		return refuse("non_finite_number", literal+" overflows a 64-bit float")
	}
	return nil
}

func (s *strictScanner) value() error {
	s.space()
	switch c := s.peek(); {
	case c == '{':
		if s.depth++; s.depth > MaxStrictDepth {
			return refuse("invalid_json", fmt.Sprintf("arrays or objects nested more than %d deep", MaxStrictDepth))
		}
		s.at++
		s.space()
		if s.peek() == '}' {
			s.at++
			s.depth--
			return nil
		}
		keys := map[string]bool{}
		for {
			s.space()
			if s.peek() != '"' {
				return refuse("invalid_json", "expected a key")
			}
			key, err := s.str(true)
			if err != nil {
				return err
			}
			if keys[key] {
				return refuse("duplicate_key", strconv.Quote(key)+" appears twice")
			}
			keys[key] = true
			s.space()
			if s.peek() != ':' {
				return refuse("invalid_json", `expected ":"`)
			}
			s.at++
			if err := s.value(); err != nil {
				return err
			}
			s.space()
			switch s.peek() {
			case ',':
				s.at++
			case '}':
				s.at++
				s.depth--
				return nil
			default:
				return refuse("invalid_json", `expected "," or "}"`)
			}
		}
	case c == '[':
		if s.depth++; s.depth > MaxStrictDepth {
			return refuse("invalid_json", fmt.Sprintf("arrays or objects nested more than %d deep", MaxStrictDepth))
		}
		s.at++
		s.space()
		if s.peek() == ']' {
			s.at++
			s.depth--
			return nil
		}
		for {
			if err := s.value(); err != nil {
				return err
			}
			s.space()
			switch s.peek() {
			case ',':
				s.at++
			case ']':
				s.at++
				s.depth--
				return nil
			default:
				return refuse("invalid_json", `expected "," or "]"`)
			}
		}
	case c == '"':
		_, err := s.str(false)
		return err
	case c == '-' || (c >= '0' && c <= '9'):
		return s.number()
	}
	for _, literal := range []string{"true", "false", "null"} {
		if len(s.text)-s.at >= len(literal) && string(s.text[s.at:s.at+len(literal)]) == literal {
			s.at += len(literal)
			return nil
		}
	}
	return refuse("invalid_json", "an unexpected token")
}

// CheckStrictJSON returns a *StrictJSONError unless raw is JSON every
// implementation reads alike (v1.2.0).
func CheckStrictJSON(raw []byte) error {
	// v1.3.0: text that is not UTF-8 is refused before any other fault, as every
	// implementation that decodes the text first refuses it.
	if !utf8.Valid(raw) {
		return refuse("invalid_json", "text that is not UTF-8")
	}
	s := &strictScanner{text: raw}
	if err := s.value(); err != nil {
		return err
	}
	s.space()
	if s.at != len(s.text) {
		return refuse("invalid_json", "text after the value")
	}
	return nil
}
