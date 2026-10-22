package forms

import (
	"strings"
	"testing"

	"github.com/go-pdfkit/reader"
)

// writeForm builds a document from fields using the writing half, with one
// widget per field merged into the field dictionary, which is the shape a
// single-widget field takes in nearly every real file.
func writeForm(t *testing.T, fields []Field) *Form {
	t.Helper()
	for _, f := range fields {
		if err := Check(f); err != nil {
			t.Fatalf("Check refused a field the test means to be valid: %v", err)
		}
	}
	d := formDoc(t, func(w *reader.Writer, page reader.Ref) (reader.Dict, reader.Array) {
		var refs reader.Array
		var annots reader.Array
		for _, f := range fields {
			dict := Dict(f)
			dict["Type"] = reader.Name("Annot")
			dict["Subtype"] = reader.Name("Widget")
			dict["P"] = page
			r := f.Widgets[0].Rect
			dict["Rect"] = nums(r[0], r[1], r[2], r[3])
			if on := f.Widgets[0].On; on != "" {
				dict["AS"] = reader.Name(on)
				dict["AP"] = reader.Dict{"N": reader.Dict{
					reader.Name(on): w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte("")}),
					"Off":           w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte("")}),
				}}
			}
			ref := w.Add(dict)
			refs = append(refs, ref)
			annots = append(annots, ref)
		}
		return reader.Dict{
			"Fields": refs,
			"DA":     str("/Helv 0 Tf 0 g"),
			"DR":     helv(w),
		}, annots
	})
	f, ok := Read(d)
	if !ok {
		t.Fatal("a form that was just written was read as no form at all")
	}
	return f
}

// ⛔ THE ORACLE OF THE WRITING HALF IS THE READING HALF. A writer tested
// against its own expectations agrees with itself: this one is tested by being
// read back by the parser that already has a corpus behind it, so a field
// written as something the format spells differently shows up as the WRONG
// KIND rather than as a passing test.
func TestEveryFieldKindSurvivesARoundTrip(t *testing.T) {
	want := []Field{
		{Name: "plain", Kind: Text, Value: "Mozart", Widgets: []Widget{{Page: 1, Rect: [4]float64{10, 10, 100, 30}}}},
		{Name: "note", Kind: Text, Multiline: true, Widgets: []Widget{{Page: 1, Rect: [4]float64{10, 40, 100, 90}}}},
		{Name: "secret", Kind: Text, Password: true, Widgets: []Widget{{Page: 1, Rect: [4]float64{10, 95, 100, 115}}}},
		{Name: "code", Kind: Text, Comb: true, MaxLen: 6, Widgets: []Widget{{Page: 1, Rect: [4]float64{10, 120, 100, 140}}}},
		{Name: "agree", Kind: Checkbox, Widgets: []Widget{{Page: 1, Rect: [4]float64{110, 10, 125, 25}, On: "Yes"}}},
		{Name: "size", Kind: ComboBox, Options: []Option{{Value: "s", Label: "Small"}, {Value: "l", Label: "Large"}},
			Widgets: []Widget{{Page: 1, Rect: [4]float64{110, 30, 190, 50}}}},
		{Name: "tags", Kind: ListBox, MultiSelect: true, Options: []Option{{Value: "a"}, {Value: "b"}},
			Widgets: []Widget{{Page: 1, Rect: [4]float64{110, 55, 190, 95}}}},
		{Name: "go", Kind: PushButton, Widgets: []Widget{{Page: 1, Rect: [4]float64{110, 100, 190, 120}}}},
		{Name: "sign", Kind: Signature, Widgets: []Widget{{Page: 1, Rect: [4]float64{110, 125, 190, 150}}}},
	}
	f := writeForm(t, want)

	if got := len(f.Fields()); got != len(want) {
		t.Fatalf("wrote %d fields and read %d back", len(want), got)
	}
	for _, w := range want {
		got, ok := f.Field(w.Name)
		if !ok {
			t.Errorf("%q was written and cannot be found", w.Name)
			continue
		}
		if got.Kind != w.Kind {
			t.Errorf("%q was written as a %s and read back as a %s", w.Name, w.Kind, got.Kind)
		}
		if got.Multiline != w.Multiline || got.Password != w.Password || got.Comb != w.Comb {
			t.Errorf("%q: text flags came back multiline=%v password=%v comb=%v",
				w.Name, got.Multiline, got.Password, got.Comb)
		}
		if got.MultiSelect != w.MultiSelect || got.Editable != w.Editable || got.Sorted != w.Sorted {
			t.Errorf("%q: choice flags came back multiselect=%v editable=%v sorted=%v",
				w.Name, got.MultiSelect, got.Editable, got.Sorted)
		}
		if got.MaxLen != w.MaxLen {
			t.Errorf("%q: MaxLen came back %d, wrote %d", w.Name, got.MaxLen, w.MaxLen)
		}
		if len(got.Options) != len(w.Options) {
			t.Errorf("%q: wrote %d options and read %d", w.Name, len(w.Options), len(got.Options))
		}
	}
}

// An option written as value+label must come back as both; one written as a
// bare value must come back with the label equal to it, which is how the
// reader presents a single-string row.
func TestAChoiceFieldsRowsSurviveWithTheirLabels(t *testing.T) {
	f := writeForm(t, []Field{{
		Name: "size", Kind: ComboBox,
		Options: []Option{{Value: "s", Label: "Small"}, {Value: "plain"}},
		Widgets: []Widget{{Page: 1, Rect: [4]float64{10, 10, 100, 30}}},
	}})
	got, ok := f.Field("size")
	if !ok {
		t.Fatal("the field is gone")
	}
	if len(got.Options) != 2 {
		t.Fatalf("read %d options", len(got.Options))
	}
	if got.Options[0].Value != "s" || got.Options[0].Label != "Small" {
		t.Errorf("the labelled row came back %+v", got.Options[0])
	}
	if got.Options[1].Value != "plain" || got.Options[1].Label != "plain" {
		t.Errorf("the bare row came back %+v", got.Options[1])
	}
}

// The flags that belong to a choice field: editable (a combo box that takes
// something off the list), sorted, and multi-select.
func TestTheChoiceFlagsSurviveTheRoundTrip(t *testing.T) {
	rect := [4]float64{10, 10, 100, 30}
	opts := []Option{{Value: "a"}, {Value: "b"}}
	for _, c := range []struct {
		why   string
		field Field
	}{
		{"an editable combo", Field{Name: "f", Kind: ComboBox, Editable: true, Options: opts,
			Widgets: []Widget{{Page: 1, Rect: rect}}}},
		{"a sorted combo", Field{Name: "f", Kind: ComboBox, Sorted: true, Options: opts,
			Widgets: []Widget{{Page: 1, Rect: rect}}}},
		{"a sorted list", Field{Name: "f", Kind: ListBox, Sorted: true, Options: opts,
			Widgets: []Widget{{Page: 1, Rect: rect}}}},
		{"an editable and sorted combo", Field{Name: "f", Kind: ComboBox, Editable: true, Sorted: true,
			Options: opts, Widgets: []Widget{{Page: 1, Rect: rect}}}},
	} {
		form := writeForm(t, []Field{c.field})
		got, ok := form.Field("f")
		if !ok {
			t.Errorf("%s: the field is gone", c.why)
			continue
		}
		if got.Editable != c.field.Editable || got.Sorted != c.field.Sorted {
			t.Errorf("%s: came back editable=%v sorted=%v, wrote editable=%v sorted=%v",
				c.why, got.Editable, got.Sorted, c.field.Editable, c.field.Sorted)
		}
		if got.Kind != c.field.Kind {
			t.Errorf("%s: came back as a %s", c.why, got.Kind)
		}
	}
}

// A list box with several rows chosen writes its value as a LIST, which is the
// one place a field's value is not a single string.
func TestAListBoxWithSeveralRowsChosenWritesThemAll(t *testing.T) {
	form := writeForm(t, []Field{{
		Name: "tags", Kind: ListBox, MultiSelect: true,
		Options: []Option{{Value: "a"}, {Value: "b"}, {Value: "c"}},
		Values:  []string{"a", "c"},
		Widgets: []Widget{{Page: 1, Rect: [4]float64{10, 10, 100, 60}}},
	}})
	got, ok := form.Field("tags")
	if !ok {
		t.Fatal("the field is gone")
	}
	if len(got.Values) != 2 || got.Values[0] != "a" || got.Values[1] != "c" {
		t.Errorf("the chosen rows came back as %v", got.Values)
	}
}

// A choice field with one row chosen writes a single string, the other arm of
// the same branch.
func TestAChoiceFieldWithOneRowChosenWritesItAsAString(t *testing.T) {
	form := writeForm(t, []Field{{
		Name: "size", Kind: ComboBox, Value: "l",
		Options: []Option{{Value: "s"}, {Value: "l"}},
		Widgets: []Widget{{Page: 1, Rect: [4]float64{10, 10, 100, 30}}},
	}})
	got, ok := form.Field("size")
	if !ok {
		t.Fatal("the field is gone")
	}
	if got.Value != "l" {
		t.Errorf("the chosen row came back as %q", got.Value)
	}
}

// Alignment is written only when it is not the default, and read back as asked.
func TestTheAlignmentSurvivesTheRoundTrip(t *testing.T) {
	for _, q := range []int{0, 1, 2} {
		form := writeForm(t, []Field{{
			Name: "f", Kind: Text, Quadding: q,
			Widgets: []Widget{{Page: 1, Rect: [4]float64{10, 10, 100, 30}}},
		}})
		got, ok := form.Field("f")
		if !ok {
			t.Fatalf("alignment %d: the field is gone", q)
		}
		if got.Quadding != q {
			t.Errorf("wrote alignment %d and read back %d", q, got.Quadding)
		}
	}
}

// The three flags every kind of field may carry go through independently of
// the kind's own bits.
func TestTheSharedFlagsSurviveOnEveryKind(t *testing.T) {
	for _, k := range []Kind{Text, Checkbox, Radio, PushButton, ComboBox, ListBox, Signature} {
		fld := Field{Name: "f", Kind: k, ReadOnly: true, Required: true, NoExport: true,
			Widgets: []Widget{{Page: 1, Rect: [4]float64{10, 10, 100, 30}, On: "Yes"}}}
		if k == ComboBox || k == ListBox {
			fld.Options = []Option{{Value: "a"}}
		}
		f := writeForm(t, []Field{fld})
		got, ok := f.Field("f")
		if !ok {
			t.Errorf("%s: the field is gone", k)
			continue
		}
		if !got.ReadOnly || !got.Required || !got.NoExport {
			t.Errorf("%s: came back readonly=%v required=%v noexport=%v",
				k, got.ReadOnly, got.Required, got.NoExport)
		}
		if got.Kind != k {
			t.Errorf("%s came back as %s once the shared flags were set", k, got.Kind)
		}
	}
}

// ⛔ A FLAG THAT DOES NOT BELONG TO THE KIND IS BIT 13 OF SOMETHING ELSE.
// Multiline is bit 13 of a text field and NoToggleToOff is bit 15 of a button;
// writing the caller's flags blindly would make a checkbox that cannot be
// unticked out of a caller who set one field struct and reused it.
func TestAFlagFromAnotherKindIsNotWritten(t *testing.T) {
	cb := Field{Name: "agree", Kind: Checkbox, Multiline: true, Password: true, Comb: true, MaxLen: 4,
		Widgets: []Widget{{Page: 1, Rect: [4]float64{10, 10, 30, 30}, On: "Yes"}}}
	if got := Flags(cb); got != 0 {
		t.Errorf("a checkbox carrying text flags was written with Ff=%d, want 0", got)
	}
	// And the same in the other direction: a text field does not get a button's.
	tx := Field{Name: "name", Kind: Text, Widgets: []Widget{{Page: 1, Rect: [4]float64{10, 10, 100, 30}}}}
	if got := Flags(tx) & (flagRadio | flagPushButton | flagNoToggleOff); got != 0 {
		t.Errorf("a text field was written with button bits set: %d", got)
	}
}

// Every refusal, because each one is a file that would open and be broken
// rather than a file that fails to be written.
func TestCheckRefusesWhatCannotBeFilledIn(t *testing.T) {
	ok := Widget{Page: 1, Rect: [4]float64{10, 10, 100, 30}}
	for _, c := range []struct {
		why   string
		field Field
		want  string
	}{
		{"no name", Field{Kind: Text, Widgets: []Widget{ok}}, "no name"},
		{"a combo with no rows", Field{Name: "a", Kind: ComboBox, Widgets: []Widget{ok}}, "no options"},
		{"a list with no rows", Field{Name: "a", Kind: ListBox, Widgets: []Widget{ok}}, "no options"},
		{"a checkbox with no on-state", Field{Name: "a", Kind: Checkbox, Widgets: []Widget{ok}}, "no on-state"},
		{"a checkbox whose state is Off", Field{Name: "a", Kind: Checkbox,
			Widgets: []Widget{{Page: 1, Rect: ok.Rect, On: "Off"}}}, "reserves"},
		{"a comb with no MaxLen", Field{Name: "a", Kind: Text, Comb: true, Widgets: []Widget{ok}}, "MaxLen"},
		{"an alignment that is not one", Field{Name: "a", Kind: Text, Quadding: 3, Widgets: []Widget{ok}}, "alignment"},
		{"an empty rectangle", Field{Name: "a", Kind: Text,
			Widgets: []Widget{{Page: 1, Rect: [4]float64{10, 10, 10, 30}}}}, "empty rectangle"},
		{"a page before the first", Field{Name: "a", Kind: Text,
			Widgets: []Widget{{Page: 0, Rect: ok.Rect}}}, "count from one"},
	} {
		err := Check(c.field)
		if err == nil {
			t.Errorf("%s was accepted", c.why)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: the refusal says %q, which does not mention %q", c.why, err, c.want)
		}
	}
	if err := Check(Field{Name: "a", Kind: Text, Widgets: []Widget{ok}}); err != nil {
		t.Errorf("a field that is fine was refused: %v", err)
	}
}

