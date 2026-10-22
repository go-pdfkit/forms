package forms

import (
	"fmt"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/go-pdfkit/reader"
)

// text encodes a string the way decodeText reads one back.
//
// ⛔ WRITING THE GO BYTES IS WRONG FOR EVERY NAME THAT IS NOT ASCII, and
// silently so. decodeText treats a string without the mark at the front as
// PDFDocEncoding and turns each BYTE into a rune, so "café" — four runes, five
// UTF-8 bytes — is read back as "cafÃ©". Found by fuzzing the round trip, on a
// name that could not be found after being written; every accented or
// non-Latin field name in the fleet would have had the same fate, and the file
// would have opened and looked fine.
//
// So: plain bytes while the string is ASCII, which the reader returns
// unchanged, and UTF-16 big-endian behind the mark otherwise.
func text(s string) reader.String {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return reader.String(s)
	}
	out := []byte{0xFE, 0xFF}
	for _, u := range utf16.Encode([]rune(s)) {
		out = append(out, byte(u>>8), byte(u))
	}
	return reader.String(out)
}

// This file is the writing half: what a field's type and flags must be for the
// reader in form.go to read back the field that was asked for.
//
// ⛔ IT SHARES THE READER'S CONSTANTS DELIBERATELY. The flag bits are the one
// thing here a second copy would get subtly wrong — bit 22 and bit 25 are the
// two the tables make easy to miscount — and a writer that disagreed with the
// reader would produce a form this package itself reads as something else,
// which no test of either half alone could see. FieldType and Flags are the
// inverse of kindOf and makeField, and TestEveryFieldKindSurvivesARoundTrip
// holds them to it.

// FieldType is the value of a field's FT entry: the four types the format has,
// as opposed to the seven kinds a person thinks in.
//
// A push button, a radio button and a checkbox are all "Btn", and a drop-down
// and a list box are both "Ch"; the flags tell them apart, which is why Flags
// and FieldType have to be written together.
func FieldType(k Kind) reader.Name {
	switch k {
	case Checkbox, Radio, PushButton:
		return "Btn"
	case ComboBox, ListBox:
		return "Ch"
	case Signature:
		return "Sig"
	}
	return "Tx"
}

// Flags is the value of a field's Ff entry for the field given: the kind's own
// bits, plus the ones the caller asked for that the kind allows.
//
// ⛔ A FLAG THAT DOES NOT BELONG TO THE KIND IS DROPPED, not written. Multiline
// on a checkbox is bit 13 of a button, which the format reads as
// NoToggleToOff — so writing "the flags the caller set" would turn a harmless
// mistake into a checkbox that cannot be unticked.
func Flags(f Field) int64 {
	var out int64
	if f.ReadOnly {
		out |= flagReadOnly
	}
	if f.Required {
		out |= flagRequired
	}
	if f.NoExport {
		out |= flagNoExport
	}
	switch f.Kind {
	case Text:
		if f.Multiline {
			out |= flagMultiline
		}
		if f.Password {
			out |= flagPassword
		}
		if f.Comb {
			out |= flagComb
		}
	case PushButton:
		out |= flagPushButton
	case Radio:
		out |= flagRadio
	case ComboBox:
		out |= flagCombo
		if f.Editable {
			out |= flagEdit
		}
		if f.Sorted {
			out |= flagSort
		}
	case ListBox:
		if f.MultiSelect {
			out |= flagMultiSelect
		}
		if f.Sorted {
			out |= flagSort
		}
	}
	return out
}

// Check says why a field cannot be written, or nothing.
//
// ⛔ IT REFUSES BEFORE A DOCUMENT IS BUILT rather than after. A field with no
// name cannot be filled, addressed or read back — the reader keys fields by
// name — and a choice field with no options is a control a person cannot use.
// Both produce a file that opens and is simply broken, which is the worst
// outcome available here.
func Check(f Field) error {
	if f.Name == "" {
		return fmt.Errorf("a field has no name, and a field is addressed by its name")
	}
	// ⛔ TEXT THAT IS NOT VALID UTF-8 CANNOT BE WRITTEN AND READ BACK. A PDF
	// text string is either PDFDocEncoding or UTF-16, and encoding invalid
	// bytes to UTF-16 replaces each with U+FFFD — so the field would be stored
	// under a name nobody can ask for. Refusing is the honest answer; writing
	// one that silently differs is not.
	for what, s := range map[string]string{"name": f.Name, "value": f.Value} {
		if !utf8.ValidString(s) {
			return fmt.Errorf("the %s of field %q is not valid UTF-8, so it cannot be written as PDF text", what, f.Name)
		}
	}
	for i, o := range f.Options {
		if !utf8.ValidString(o.Value) || !utf8.ValidString(o.Label) {
			return fmt.Errorf("option %d of field %q is not valid UTF-8", i+1, f.Name)
		}
	}
	switch f.Kind {
	case ComboBox, ListBox:
		if len(f.Options) == 0 {
			return fmt.Errorf("the choice field %q has no options to choose from", f.Name)
		}
	case Checkbox, Radio:
		for i, w := range f.Widgets {
			if w.On == "" {
				return fmt.Errorf("widget %d of %q has no on-state name, so nothing can tell it apart from the others", i+1, f.Name)
			}
			if w.On == "Off" {
				return fmt.Errorf("widget %d of %q is named Off, which is the name the format reserves for NOT chosen", i+1, f.Name)
			}
		}
	}
	if f.Comb && f.MaxLen <= 0 {
		return fmt.Errorf("the comb field %q has no MaxLen, and a comb field is divided into MaxLen cells", f.Name)
	}
	if f.Quadding < 0 || f.Quadding > 2 {
		return fmt.Errorf("field %q asks for alignment %d, which is not 0, 1 or 2", f.Name, f.Quadding)
	}
	// ⛔ A radio group is several widgets of ONE field, told apart only by
	// their on-state names. Two sharing a name is a group where pressing one
	// lights both and nothing can say which was chosen.
	if f.Kind == Radio || f.Kind == Checkbox {
		seen := map[string]bool{}
		for i, w := range f.Widgets {
			if seen[w.On] {
				return fmt.Errorf("widget %d of %q is in state %q, which another widget of the same field already uses", i+1, f.Name, w.On)
			}
			seen[w.On] = true
		}
	}
	for i, w := range f.Widgets {
		if w.Rect[2] <= w.Rect[0] || w.Rect[3] <= w.Rect[1] {
			return fmt.Errorf("widget %d of %q has an empty rectangle %v", i+1, f.Name, w.Rect)
		}
		if w.Page < 1 {
			return fmt.Errorf("widget %d of %q is on page %d; pages count from one", i+1, f.Name, w.Page)
		}
	}
	return nil
}

// CheckAll says why a set of fields cannot be written, or nothing. It is what
// a caller building a whole form wants: Check on each, plus the one rule that
// is about the set rather than about any field in it.
//
// ⛔ A NAME USED TWICE IS A SILENT LOSS, not a broken file. The reader keys
// fields by name, so the second shadows the first: the document carries both
// widgets, the form offers one field, and whoever fills it in has no way to
// see that half of what they typed went nowhere.
func CheckAll(fields []Field) error {
	seen := map[string]int{}
	for i, f := range fields {
		if err := Check(f); err != nil {
			return err
		}
		if first, ok := seen[f.Name]; ok {
			return fmt.Errorf("fields %d and %d are both named %q, and a form addresses a field by its name", first+1, i+1, f.Name)
		}
		seen[f.Name] = i
	}
	return nil
}

// Dict is a field's dictionary, without the entries only a writer can fill in:
// the widget references under Kids, and the page reference under each widget.
//
// The caller adds those, because allocating object numbers belongs to whoever
// is writing the file.
func Dict(f Field) reader.Dict {
	d := reader.Dict{
		"FT": FieldType(f.Kind),
		"T":  text(f.Name),
	}
	if fl := Flags(f); fl != 0 {
		d["Ff"] = reader.Integer(fl)
	}
	if f.Quadding != 0 {
		d["Q"] = reader.Integer(int64(f.Quadding))
	}
	if f.MaxLen > 0 && f.Kind == Text {
		d["MaxLen"] = reader.Integer(int64(f.MaxLen))
	}
	switch f.Kind {
	case ComboBox, ListBox:
		opts := make(reader.Array, 0, len(f.Options))
		for _, o := range f.Options {
			// A row whose label is its value is written as one string, which is
			// what the reader expects and what every producer writes.
			if o.Label == "" || o.Label == o.Value {
				opts = append(opts, text(o.Value))
				continue
			}
			opts = append(opts, reader.Array{text(o.Value), text(o.Label)})
		}
		d["Opt"] = opts
		if len(f.Values) > 0 {
			vals := make(reader.Array, 0, len(f.Values))
			for _, v := range f.Values {
				vals = append(vals, text(v))
			}
			d["V"] = vals
		} else if f.Value != "" {
			d["V"] = text(f.Value)
		}
	case Checkbox, Radio:
		// A button's value is the NAME of the chosen state, not a string, and
		// "Off" is the one that means nothing is chosen.
		v := f.Value
		if v == "" {
			v = "Off"
		}
		d["V"] = reader.Name(v)
	case PushButton, Signature:
		// Neither holds a value.
	default:
		if f.Value != "" {
			d["V"] = text(f.Value)
		}
	}
	return d
}
