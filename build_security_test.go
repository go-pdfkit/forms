package forms

import (
	"strings"
	"testing"
)

// ⛔ A FIELD NAME IS UNTRUSTED TEXT. It arrives from whoever describes the
// form, and it is written into PDF syntax, where ")" ends a string and "\"
// escapes the next byte. A name carrying those would, if they went through
// unescaped, close the string early and leave the rest of the name being read
// as DICTIONARY SYNTAX — the PDF equivalent of an injection, which would let a
// form description add entries to objects it does not own.
//
// This is the one test here whose oracle is that the document still reads
// correctly: if escaping were wrong, the field would come back under a
// different name, or the file would not parse at all.
func TestAFieldNameCarryingPDFSyntaxIsWrittenSafely(t *testing.T) {
	for _, name := range []string{
		`ends)here`,
		`back\slash`,
		`both)and\together`,
		`/Type /Catalog`,
		`a(nested)pair`,
		"with\nnewline",
		"with\ttab",
		`>>endobj`,
		`parens((((unbalanced`,
	} {
		f := writeForm(t, []Field{{
			Name: name, Kind: Text,
			Widgets: []Widget{{Page: 1, Rect: [4]float64{10, 10, 100, 30}}},
		}})
		got, ok := f.Field(name)
		if !ok {
			var have []string
			for _, fl := range f.Fields() {
				have = append(have, fl.Name)
			}
			t.Errorf("a field named %q came back as %q", name, strings.Join(have, ", "))
			continue
		}
		if got.Name != name {
			t.Errorf("wrote %q and read back %q", name, got.Name)
		}
	}
}

// The same for a choice field's rows, which are strings from the same source.
func TestAnOptionCarryingPDFSyntaxIsWrittenSafely(t *testing.T) {
	f := writeForm(t, []Field{{
		Name: "pick", Kind: ComboBox,
		Options: []Option{{Value: `v)alue`, Label: `l\abel`}, {Value: `(both)`}},
		Widgets: []Widget{{Page: 1, Rect: [4]float64{10, 10, 100, 30}}},
	}})
	got, ok := f.Field("pick")
	if !ok {
		t.Fatal("the field is gone")
	}
	if len(got.Options) != 2 {
		t.Fatalf("read %d options", len(got.Options))
	}
	if got.Options[0].Value != `v)alue` || got.Options[0].Label != `l\abel` {
		t.Errorf("the first row came back %+v", got.Options[0])
	}
	if got.Options[1].Value != `(both)` {
		t.Errorf("the second row came back %+v", got.Options[1])
	}
}

// ⛔ A NAME THAT IS NOT ASCII MUST COME BACK AS ITSELF. decodeText reads a
// string without the mark at the front as PDFDocEncoding — one rune per
// BYTE — so writing Go's UTF-8 bytes turns "café" into "cafÃ©" in a file that
// opens and looks perfectly well formed. Found by fuzzing the round trip; it
// would have hit every accented or non-Latin field name in the fleet.
func TestANonASCIINameSurvivesTheRoundTrip(t *testing.T) {
	for _, name := range []string{
		"café",
		"Grüße",
		"日本語のフィールド",
		"Ωμέγα",
		"эхо",
		"emoji 🙂 here",
		"mixed café 日本 ok",
	} {
		f := writeForm(t, []Field{{
			Name: name, Kind: Text, Value: name,
			Widgets: []Widget{{Page: 1, Rect: [4]float64{10, 10, 100, 30}}},
		}})
		got, ok := f.Field(name)
		if !ok {
			var have []string
			for _, fl := range f.Fields() {
				have = append(have, fl.Name)
			}
			t.Errorf("wrote the name %q and read back %q", name, strings.Join(have, ", "))
			continue
		}
		if got.Value != name {
			t.Errorf("wrote the value %q and read back %q", name, got.Value)
		}
	}
}

// Text that is not valid UTF-8 cannot be written as PDF text and read back, so
// it is refused rather than stored under a name nobody can ask for.
func TestTextThatIsNotValidUTF8IsRefused(t *testing.T) {
	rect := [4]float64{10, 10, 100, 30}
	bad := "\xff\xfe"
	for _, c := range []struct {
		why   string
		field Field
	}{
		{"a name", Field{Name: bad, Kind: Text, Widgets: []Widget{{Page: 1, Rect: rect}}}},
		{"a value", Field{Name: "ok", Kind: Text, Value: bad, Widgets: []Widget{{Page: 1, Rect: rect}}}},
		{"an option", Field{Name: "ok", Kind: ComboBox, Options: []Option{{Value: bad}},
			Widgets: []Widget{{Page: 1, Rect: rect}}}},
	} {
		if err := Check(c.field); err == nil {
			t.Errorf("%s that is not valid UTF-8 was accepted", c.why)
		}
	}
}

// ⛔ TWO FIELDS OF ONE NAME IS A SILENT LOSS. The reader keys fields by name,
// so the second overwrites the first: the document holds both, the form shows
// one, and a person filling it in cannot tell. That is the shape of defect this
// fleet has written down more than once, so it is refused rather than written.
func TestTwoFieldsOfTheSameNameAreRefused(t *testing.T) {
	rect := [4]float64{10, 10, 100, 30}
	fields := []Field{
		{Name: "name", Kind: Text, Widgets: []Widget{{Page: 1, Rect: rect}}},
		{Name: "name", Kind: Checkbox, Widgets: []Widget{{Page: 1, Rect: rect, On: "Yes"}}},
	}
	err := CheckAll(fields)
	if err == nil {
		t.Fatal("two fields of one name were accepted")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("the refusal does not name the field: %v", err)
	}
	// And a set with no duplicate passes, so the check is about duplication
	// rather than about there being more than one field.
	fields[1].Name = "agree"
	if err := CheckAll(fields); err != nil {
		t.Errorf("a set with distinct names was refused: %v", err)
	}
}

// ⛔ CheckAll RUNS Check ON EACH, or a caller who uses the set-level check
// gets none of the per-field refusals and writes a broken file believing it
// was checked.
func TestCheckAllAlsoChecksEachField(t *testing.T) {
	err := CheckAll([]Field{{Name: "", Kind: Text,
		Widgets: []Widget{{Page: 1, Rect: [4]float64{10, 10, 100, 30}}}}})
	if err == nil {
		t.Fatal("a nameless field passed the set-level check")
	}
}

// A radio group is several widgets of ONE field, each with its own on-state,
// and two widgets sharing a state name cannot be told apart once pressed.
func TestARadioGroupWithTwoWidgetsOfTheSameStateIsRefused(t *testing.T) {
	f := Field{Name: "size", Kind: Radio, Widgets: []Widget{
		{Page: 1, Rect: [4]float64{10, 10, 25, 25}, On: "S"},
		{Page: 1, Rect: [4]float64{30, 10, 45, 25}, On: "S"},
	}}
	if err := Check(f); err == nil {
		t.Fatal("two radio widgets of one state were accepted")
	}
	f.Widgets[1].On = "L"
	if err := Check(f); err != nil {
		t.Errorf("a well-formed radio group was refused: %v", err)
	}
}
