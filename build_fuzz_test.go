package forms

import (
	"testing"
)

// FuzzAFieldCheckAcceptsSurvivesTheRoundTrip holds the one promise the writing
// half makes: whatever Check accepts can be written, read back, and is the
// same field.
//
// ⛔ THE PROPERTY IS TIED TO Check, not to a list of inputs I thought of. A
// name is arbitrary text from whoever describes the form, and the interesting
// inputs here are the ones nobody writes by hand — unbalanced parentheses, a
// lone backslash at the end, NUL, invalid UTF-8 — each of which is a way a
// string can escape its delimiters and be read as syntax.
func FuzzAFieldCheckAcceptsSurvivesTheRoundTrip(f *testing.F) {
	f.Add("name", uint8(0), "value", "opt")
	f.Add(`ends)here`, uint8(4), "", "a")
	f.Add(`trailing\`, uint8(0), "", "")
	f.Add("\x00nul", uint8(1), "", "")
	f.Add("\xff\xfe", uint8(5), "", "x")

	f.Fuzz(func(t *testing.T, name string, kind uint8, value, opt string) {
		k := Kind(kind % 7)
		fld := Field{
			Name:    name,
			Kind:    k,
			Value:   value,
			Widgets: []Widget{{Page: 1, Rect: [4]float64{10, 10, 100, 30}}},
		}
		switch k {
		case Checkbox, Radio:
			fld.Widgets[0].On = "Yes"
			fld.Value = ""
		case ComboBox, ListBox:
			fld.Options = []Option{{Value: opt}}
		}
		if err := Check(fld); err != nil {
			return // refused on purpose; the promise is about what it accepts
		}

		// Dict must not panic, and must name the field it was given.
		d := Dict(fld)
		if d["FT"] != FieldType(k) {
			t.Fatalf("Dict wrote FT %v for a %s", d["FT"], k)
		}

		form := writeForm(t, []Field{fld})
		got, ok := form.Field(name)
		if !ok {
			t.Fatalf("a field named %q was written and cannot be found", name)
		}
		if got.Name != name {
			t.Fatalf("wrote the name %q and read back %q", name, got.Name)
		}
		if got.Kind != k {
			t.Fatalf("wrote a %s and read back a %s", k, got.Kind)
		}
	})
}
