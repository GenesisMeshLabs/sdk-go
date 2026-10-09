package genesismesh

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Canonical JSON: byte-identical to Python's
// json.dumps(value, sort_keys=True, separators=(",", ":")) applied to the value
// the Network Authority parsed. Keys are sorted by code point, every
// non-ASCII character is escaped (ensure_ascii), and numbers follow Python:
// integer literals are kept as written, other numbers use Python's float repr
// (so a received 90.0 stays 90.0 and 1e-05 stays 1e-05).

// CanonicalJSON returns the canonical form of a JSON document.
func CanonicalJSON(raw []byte) (string, error) {
	v, err := decodeJSON(raw)
	if err != nil {
		return "", err
	}
	b, err := marshalCanonical(v)
	return string(b), err
}

// decodeJSON parses JSON keeping number literals (json.Number), so canonical
// forms of received artifacts are reproduced exactly. v1.2.0: JSON every
// implementation would not read alike is refused (*StrictJSONError).
func decodeJSON(raw []byte) (interface{}, error) {
	if err := CheckStrictJSON(raw); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, fmt.Errorf("genesismesh: trailing data after JSON value")
	}
	return v, nil
}

func marshalCanonical(v interface{}) ([]byte, error) {
	switch val := v.(type) {
	case nil:
		return []byte("null"), nil
	case bool:
		if val {
			return []byte("true"), nil
		}
		return []byte("false"), nil
	case string:
		return []byte(pythonString(val)), nil
	case json.Number:
		return pythonNumberLiteral(string(val))
	case float64:
		return pythonFloat64(val)
	case float32:
		return pythonFloat64(float64(val))
	case int:
		return []byte(strconv.Itoa(val)), nil
	case int64:
		return []byte(strconv.FormatInt(val, 10)), nil
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys) // byte order of UTF-8 is code point order
		var buf bytes.Buffer
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			buf.WriteString(pythonString(k))
			buf.WriteByte(':')
			vb, err := marshalCanonical(val[k])
			if err != nil {
				return nil, err
			}
			buf.Write(vb)
		}
		buf.WriteByte('}')
		return buf.Bytes(), nil
	case []interface{}:
		var buf bytes.Buffer
		buf.WriteByte('[')
		for i, item := range val {
			if i > 0 {
				buf.WriteByte(',')
			}
			vb, err := marshalCanonical(item)
			if err != nil {
				return nil, err
			}
			buf.Write(vb)
		}
		buf.WriteByte(']')
		return buf.Bytes(), nil
	default:
		// Structs and other Go values: encode, re-decode with literals, canonicalize.
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		generic, err := decodeJSON(raw)
		if err != nil {
			return nil, err
		}
		return marshalCanonical(generic)
	}
}

// pythonString escapes like json.dumps with ensure_ascii=True.
func pythonString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			switch {
			case r < 0x20:
				fmt.Fprintf(&b, `\u%04x`, r)
			case r < 0x7f: // printable ASCII; Python escapes DEL (0x7f) and above
				b.WriteRune(r)
			case r > 0xffff:
				r1, r2 := utf16.EncodeRune(r)
				fmt.Fprintf(&b, `\u%04x\u%04x`, r1, r2)
			default:
				fmt.Fprintf(&b, `\u%04x`, r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func isIntegerLiteral(s string) bool {
	if s == "" {
		return false
	}
	if s[0] == '-' {
		s = s[1:]
	}
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// pythonNumberLiteral: an integer literal stays as written (Python int, any size);
// anything else is a Python float.
func pythonNumberLiteral(lit string) ([]byte, error) {
	if isIntegerLiteral(lit) {
		return []byte(lit), nil
	}
	f, err := strconv.ParseFloat(lit, 64)
	if err != nil {
		return nil, fmt.Errorf("genesismesh: invalid number %q", lit)
	}
	return pythonFloatRepr(f)
}

// pythonFloat64 encodes a Go float as Python parses Go's JSON text for it.
func pythonFloat64(f float64) ([]byte, error) {
	raw, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	return pythonNumberLiteral(string(raw))
}

// pythonFloatRepr formats like Python's repr(float): shortest round-trip digits,
// positional for exponents -4..15 (always with a fraction), else d.ddde±XX.
func pythonFloatRepr(f float64) ([]byte, error) {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return nil, fmt.Errorf("genesismesh: canonical JSON cannot encode %v", f)
	}
	sci := strconv.FormatFloat(f, 'e', -1, 64) // e.g. -1.5e+16
	mantissa, expPart, _ := strings.Cut(sci, "e")
	exp, err := strconv.Atoi(expPart)
	if err != nil {
		return nil, err
	}
	negative := strings.HasPrefix(mantissa, "-")
	digits := strings.ReplaceAll(strings.TrimPrefix(mantissa, "-"), ".", "")
	sign := ""
	if negative {
		sign = "-"
	}
	if exp >= -4 && exp < 16 {
		point := exp + 1
		var out string
		switch {
		case point <= 0:
			out = "0." + strings.Repeat("0", -point) + digits
		case point >= len(digits):
			out = digits + strings.Repeat("0", point-len(digits)) + ".0"
		default:
			out = digits[:point] + "." + digits[point:]
		}
		return []byte(sign + out), nil
	}
	expSign := "+"
	if exp < 0 {
		expSign = "-"
		exp = -exp
	}
	return []byte(fmt.Sprintf("%s%se%s%02d", sign, strings.TrimPrefix(mantissa, "-"), expSign, exp)), nil
}
