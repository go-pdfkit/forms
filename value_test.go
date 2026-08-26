package forms

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-pdfkit/reader"
)

// choiceDoc is a choice field with two rows and whatever flags the test wants.
func choiceDoc(t *testing.T, flags int64) *Form {
	t.Helper()
	_, f := textFieldDoc(t, reader.Dict{
		"FT": reader.Name("Ch"), "Ff": reader.Integer(flags),
		"Opt": reader.Array{
			reader.Array{str("FR"), str("France")},
			reader.Array{str("BE"), str("Belgique")},
		},
	})
	return f
}

// buttonDoc is a checkbox whose widget names Yes as its chosen state.
func buttonDoc(t *testing.T, flags int64) *Form {
	t.Helper()
	d := formDoc(t, func(w *reader.Writer, page reader.Ref) (reader.Dict, reader.Array) {
		blank := w.Add(&reader.Stream{Dict: reader.Dict{"BBox": nums(0, 0, 10, 10)}, Raw: []byte("")})
		ref := w.Add(reader.Dict{"T": str("tick"), "FT": reader.Name("Btn"),
			"Ff": reader.Integer(flags), "Subtype": reader.Name("Widget"),
			"Rect": nums(0, 0, 12, 12), "P": page,
			"AP": reader.Dict{"N": reader.Dict{"Off": blank, "Yes": blank}}})
		return reader.Dict{"Fields": reader.Array{ref}}, reader.Array{ref}
	})
	f, ok := Read(d)
	if !ok {
		t.Fatal("no form")
	}
	return f
}

func TestPuttingAValueInATextField(t *testing.T) {
	_, f := textFieldDoc(t, nil)
	fld := f.Fields()[0]
	if err := fld.SetText("Mozart"); err != nil {
		t.Fatal(err)
	}
	if fld.Value != "Mozart" || !fld.Changed() {
		t.Errorf("holds %q, changed %v", fld.Value, fld.Changed())
	}
	if len(f.Changed()) != 1 {
		t.Errorf("the form lists %d changed fields", len(f.Changed()))
	}
}

func TestATextFieldKeepsToItsLength(t *testing.T) {
	_, f := textFieldDoc(t, reader.Dict{"MaxLen": reader.Integer(4)})
	fld := f.Fields()[0]
	if err := fld.SetText("Mozart"); err != nil {
		t.Fatal(err)
	}
	if fld.Value != "Moza" {
		t.Errorf("holds %q, wanted it cut to four", fld.Value)
	}
}

func TestASingleLineFieldHoldsOneLine(t *testing.T) {
	// A newline in a box that shows one line would run off the end of it
	// rather than wrap, so it becomes a space.
	_, f := textFieldDoc(t, nil)
	fld := f.Fields()[0]
	if err := fld.SetText("one\r\ntwo\nthree\rfour"); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(fld.Value, "\r\n") {
		t.Errorf("holds %q", fld.Value)
	}

	_, g := textFieldDoc(t, reader.Dict{"Ff": reader.Integer(flagMultiline)})
	multi := g.Fields()[0]
	if err := multi.SetText("one\ntwo"); err != nil {
		t.Fatal(err)
	}
	if multi.Value != "one\ntwo" {
		t.Errorf("a field that takes several lines holds %q", multi.Value)
	}
}

func TestAReadOnlyFieldIsNotFilledIn(t *testing.T) {
	_, f := textFieldDoc(t, reader.Dict{"Ff": reader.Integer(flagReadOnly)})
	fld := f.Fields()[0]
	for _, err := range []error{
		fld.SetText("x"), fld.SetChecked(true), fld.Press("Yes"), fld.Choose("x"),
	} {
		if !errors.Is(err, ErrReadOnly) {
			t.Errorf("a read-only field gave %v", err)
		}
	}
	if fld.Changed() {
		t.Error("a read-only field was marked as changed")
	}
}

func TestAFieldRefusesTheWrongSortOfValue(t *testing.T) {
	_, text := textFieldDoc(t, nil)
	if err := text.Fields()[0].SetChecked(true); !errors.Is(err, ErrKind) {
		t.Errorf("ticking a text field gave %v", err)
	}
	if err := text.Fields()[0].Press("Yes"); !errors.Is(err, ErrKind) {
		t.Errorf("pressing a text field gave %v", err)
	}
	if err := text.Fields()[0].Choose("a"); !errors.Is(err, ErrKind) {
		t.Errorf("choosing in a text field gave %v", err)
	}
	tick := buttonDoc(t, 0)
	if err := tick.Fields()[0].SetText("x"); !errors.Is(err, ErrKind) {
		t.Errorf("typing into a checkbox gave %v", err)
	}
}

func TestTickingABox(t *testing.T) {
	f := buttonDoc(t, 0)
	fld := f.Fields()[0]
	if err := fld.SetChecked(true); err != nil {
		t.Fatal(err)
	}
	if fld.Value != "Yes" || !fld.Checked() {
		t.Errorf("holds %q, checked %v", fld.Value, fld.Checked())
	}
	if err := fld.SetChecked(false); err != nil {
		t.Fatal(err)
	}
	if fld.Value != "Off" || fld.Checked() {
		t.Errorf("unticked, holds %q, checked %v", fld.Value, fld.Checked())
	}
}

func TestTickingABoxThatNamesNoState(t *testing.T) {
	// A widget with no appearance dictionary has no name for chosen, and the
	// format's own default is used.
	_, f := textFieldDoc(t, reader.Dict{"FT": reader.Name("Btn")})
	fld := f.Fields()[0]
	if err := fld.SetChecked(true); err != nil {
		t.Fatal(err)
	}
	if fld.Value != "Yes" {
		t.Errorf("holds %q, wanted Yes", fld.Value)
	}
}

func TestPressingOneButtonOfAGroup(t *testing.T) {
	f := buttonDoc(t, flagRadio)
	fld := f.Fields()[0]
	if err := fld.Press("Yes"); err != nil {
		t.Fatal(err)
	}
	if fld.Value != "Yes" {
		t.Errorf("holds %q", fld.Value)
	}
	if err := fld.Press("Off"); err != nil {
		t.Fatal(err)
	}
	if err := fld.Press("Nowhere"); !errors.Is(err, ErrKind) {
		t.Errorf("pressing a button that does not exist gave %v", err)
	}
}

func TestChoosingRows(t *testing.T) {
	f := choiceDoc(t, flagCombo)
	fld := f.Fields()[0]
	if err := fld.Choose("FR"); err != nil {
		t.Fatal(err)
	}
	if fld.Value != "FR" || fld.Text() != "France" {
		t.Errorf("holds %q and shows %q", fld.Value, fld.Text())
	}
	if err := fld.Choose(); err != nil {
		t.Fatal(err)
	}
	if fld.Value != "" {
		t.Errorf("after choosing nothing it holds %q", fld.Value)
	}
	if err := fld.Choose("XX"); !errors.Is(err, ErrKind) {
		t.Errorf("choosing a row that does not exist gave %v", err)
	}
	if err := fld.Choose("FR", "BE"); !errors.Is(err, ErrKind) {
		t.Errorf("choosing two rows of a field that takes one gave %v", err)
	}
}

func TestChoosingSeveralRows(t *testing.T) {
	f := choiceDoc(t, flagMultiSelect)
	fld := f.Fields()[0]
	if err := fld.Choose("FR", "BE"); err != nil {
		t.Fatal(err)
	}
	if fld.Value != "FR" || len(fld.Values) != 2 {
		t.Errorf("holds %q and %v", fld.Value, fld.Values)
	}
	if got := fld.selected(); len(got) != 2 {
		t.Errorf("chose %v", got)
	}
}

func TestAComboBoxThatTakesSomethingOfItsOwn(t *testing.T) {
	f := choiceDoc(t, flagCombo|flagEdit)
	fld := f.Fields()[0]
	if err := fld.Choose("Andorre"); err != nil {
		t.Errorf("an editable combo box refused something not on its list: %v", err)
	}
	if err := fld.SetText("Monaco"); err != nil {
		t.Errorf("an editable combo box refused typing: %v", err)
	}
	fixed := choiceDoc(t, flagCombo)
	if err := fixed.Fields()[0].SetText("Monaco"); !errors.Is(err, ErrKind) {
		t.Errorf("a fixed combo box took typing: %v", err)
	}
	if err := fixed.Fields()[0].SetText("France"); err != nil {
		t.Errorf("a fixed combo box refused one of its own rows: %v", err)
	}
}

func TestFillingAFieldByName(t *testing.T) {
	_, f := textFieldDoc(t, nil)
	if err := f.Fill("name", "Mozart"); err != nil {
		t.Fatal(err)
	}
	if f.Fields()[0].Value != "Mozart" {
		t.Errorf("holds %q", f.Fields()[0].Value)
	}
	if err := f.Fill("nothing", "x"); err == nil {
		t.Error("filling a field that does not exist was allowed")
	}
}

func TestFillingABoxByName(t *testing.T) {
	for _, c := range []struct {
		given string
		want  string
	}{
		{"yes", "Yes"}, {"true", "Yes"}, {"1", "Yes"}, {"on", "Yes"},
		{"no", "Off"}, {"false", "Off"}, {"0", "Off"}, {"", "Off"}, {"off", "Off"},
		{"Yes", "Yes"},
	} {
		f := buttonDoc(t, 0)
		if err := f.Fill("tick", c.given); err != nil {
			t.Fatalf("%q: %v", c.given, err)
		}
		if got := f.Fields()[0].Value; got != c.want {
			t.Errorf("%q left it holding %q, wanted %q", c.given, got, c.want)
		}
	}
}

func TestFillingAChoiceByName(t *testing.T) {
	f := choiceDoc(t, flagCombo)
	if err := f.Fill("name", "FR"); err != nil {
		t.Fatal(err)
	}
	if f.Fields()[0].Value != "FR" {
		t.Errorf("holds %q", f.Fields()[0].Value)
	}
}

func TestSomeFieldsAreNotThingsToFillIn(t *testing.T) {
	for _, kind := range []reader.Name{"Btn", "Sig"} {
		flags := int64(0)
		if kind == "Btn" {
			flags = flagPushButton
		}
		_, f := textFieldDoc(t, reader.Dict{"FT": kind, "Ff": reader.Integer(flags)})
		if err := f.Fill("name", "x"); !errors.Is(err, ErrKind) {
			t.Errorf("%s: filling it gave %v", kind, err)
		}
	}
}

func TestWhatAFieldShows(t *testing.T) {
	// A choice field shows the label its value stands for; everything else
	// shows its value.
	_, f := textFieldDoc(t, reader.Dict{"V": str("plain")})
	if got := f.Fields()[0].Text(); got != "plain" {
		t.Errorf("a text field shows %q", got)
	}
	c := choiceDoc(t, flagCombo)
	if err := c.Fields()[0].Choose("BE"); err != nil {
		t.Fatal(err)
	}
	if got := c.Fields()[0].Text(); got != "Belgique" {
		t.Errorf("a chosen row shows %q", got)
	}
}
