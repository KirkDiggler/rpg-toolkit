// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"

	"encoding/json/jsontext"
)

// Scalar is one bool, int or string: the operand of a fact or bag leaf and the
// value a set act writes. Nothing else can be held, by construction — there is
// no interface inside it, so a guard carried in an answer is plain data.
//
// The zero Scalar holds nothing. A leaf or act whose value is unset is refused
// when its recipe or guard compiles, and an unset Scalar refuses to encode.
//
// Two Scalars are equal (==) exactly when they hold the same kind and value,
// which is the guard's eq comparison.
//
// JSON is the bare value: true, 3, "str". Decoding refuses null, a number
// with a fraction or exponent or outside int's range, an array and an object.
type Scalar struct {
	kind ScalarKind
	b    bool
	i    int
	s    string
}

// ErrInvalidScalar is wrapped by every refusal to decode a Scalar.
var ErrInvalidScalar = errors.New("machine: a value must be a bool, an int or a string")

// BoolScalar returns the Scalar holding b.
func BoolScalar(b bool) Scalar { return Scalar{kind: KindBool, b: b} }

// IntScalar returns the Scalar holding n.
func IntScalar(n int) Scalar { return Scalar{kind: KindInt, i: n} }

// StringScalar returns the Scalar holding s.
func StringScalar(s string) Scalar { return Scalar{kind: KindString, s: s} }

// ScalarOf returns the Scalar holding v when v is a bool, an int or a string,
// and false for any other value (nil included).
func ScalarOf(v any) (Scalar, bool) {
	switch x := v.(type) {
	case bool:
		return BoolScalar(x), true
	case int:
		return IntScalar(x), true
	case string:
		return StringScalar(x), true
	}
	return Scalar{}, false
}

// Kind returns the kind held, and "" for the zero Scalar.
func (s Scalar) Kind() ScalarKind { return s.kind }

// IsZero reports whether s holds nothing.
func (s Scalar) IsZero() bool { return s.kind == "" }

// Bool returns the bool held, and false when s holds another kind.
func (s Scalar) Bool() (bool, bool) { return s.b, s.kind == KindBool }

// Int returns the int held, and false when s holds another kind.
func (s Scalar) Int() (int, bool) { return s.i, s.kind == KindInt }

// Text returns the string held, and false when s holds another kind.
func (s Scalar) Text() (string, bool) { return s.s, s.kind == KindString }

// Value returns what s holds as a bag value (bool, int or string), and nil for
// the zero Scalar.
func (s Scalar) Value() any {
	switch s.kind {
	case KindBool:
		return s.b
	case KindInt:
		return s.i
	case KindString:
		return s.s
	}
	return nil
}

// String renders s for messages: the JSON form, or "unset".
func (s Scalar) String() string {
	switch s.kind {
	case KindBool:
		return strconv.FormatBool(s.b)
	case KindInt:
		return strconv.Itoa(s.i)
	case KindString:
		return strconv.Quote(s.s)
	}
	return "unset"
}

// MarshalJSONTo writes the bare value. An unset Scalar is refused rather than
// written as null, which would not decode.
func (s Scalar) MarshalJSONTo(enc *jsontext.Encoder) error {
	switch s.kind {
	case KindBool:
		return enc.WriteToken(jsontext.Bool(s.b))
	case KindInt:
		return enc.WriteToken(jsontext.Int(int64(s.i)))
	case KindString:
		return enc.WriteToken(jsontext.String(s.s))
	}
	return fmt.Errorf("%w: the value is unset", ErrInvalidScalar)
}

// UnmarshalJSONFrom reads a bare bool, whole number or string. Anything else
// is refused, wrapping ErrInvalidScalar.
func (s *Scalar) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	tok, err := dec.ReadToken()
	if err != nil {
		return err
	}
	switch tok.Kind() {
	case 't', 'f':
		*s = BoolScalar(tok.Bool())
		return nil
	case '"':
		*s = StringScalar(tok.String())
		return nil
	case '0':
		n, err := strconv.ParseInt(tok.String(), 10, strconv.IntSize)
		if err != nil {
			return fmt.Errorf("%w: %s is not a whole number in range", ErrInvalidScalar, tok.String())
		}
		*s = IntScalar(int(n))
		return nil
	case 'n':
		return fmt.Errorf("%w: got null", ErrInvalidScalar)
	}
	if err := dec.SkipValue(); err != nil {
		return err
	}
	return fmt.Errorf("%w: got %s", ErrInvalidScalar, tok.Kind())
}

// MarshalJSON is MarshalJSONTo for encoding/json v1 callers.
func (s Scalar) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	enc := jsontext.NewEncoder(&buf)
	if err := s.MarshalJSONTo(enc); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// UnmarshalJSON is UnmarshalJSONFrom for encoding/json v1 callers.
func (s *Scalar) UnmarshalJSON(data []byte) error {
	dec := jsontext.NewDecoder(bytes.NewReader(data))
	if err := s.UnmarshalJSONFrom(dec); err != nil {
		return err
	}
	if _, err := dec.ReadToken(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: trailing data after the value", ErrInvalidScalar)
	}
	return nil
}

// kindName is the kind held, spelled as a refusal names a value's type; the
// zero Scalar reads as "<nil>", as an absent value always has.
func (s Scalar) kindName() string {
	if s.kind == "" {
		return "<nil>"
	}
	return string(s.kind)
}
