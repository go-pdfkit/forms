package forms

import (
	"strings"
	"testing"

	"github.com/go-pdfkit/reader"
)

func TestADocumentWithNoFormSaysSo(t *testing.T) {
	// Four ways a document has no form: no AcroForm at all; one with no
	// field list; one whose list is empty; and one whose list holds nothing
	// that is a field. The last is not a curiosity — 561 of the 118 833 files
	// in the figure corpus are exactly that, an AcroForm dictionary a
	// producer left behind with an empty list inside it.
	for _, c := range []struct {
		why  string
		form reader.Dict
	}{
		{"no AcroForm at all", nil},
		{"an AcroForm with no field list", reader.Dict{"DA": str("/Helv 0 Tf")}},
		{"an AcroForm whose list is empty", reader.Dict{"Fields": reader.Array{}}},
		{"a list holding something that is not a field", reader.Dict{
			"Fields": reader.Array{reader.Integer(7)}}},
		{"a list holding a node with no type anywhere", reader.Dict{
			"Fields": reader.Array{reader.Dict{"T": str("nameless")}}}},
	} {
		d := formDoc(t, func(w *reader.Writer, page reader.Ref) (reader.Dict, reader.Array) {
			return c.form, nil
		})
		if _, ok := Read(d); ok {
			t.Errorf("%s: was read as a form", c.why)
		}
	}
}

func TestAFieldIsReadWithItsWidget(t *testing.T) {
	_, f := textFieldDoc(t, reader.Dict{"V": str("Mozart")})
	fields := f.Fields()
	if len(fields) != 1 {
		t.Fatalf("read %d fields, wanted one", len(fields))
	}
	fld := fields[0]
	if fld.Name != "name" || fld.Kind != Text || fld.Value != "Mozart" {
		t.Errorf("read %q, a %s, holding %q", fld.Name, fld.Kind, fld.Value)
	}
	if len(fld.Widgets) != 1 {
		t.Fatalf("read %d widgets, wanted one", len(fld.Widgets))
	}
	w := fld.Widgets[0]
	if w.Page != 1 {
		t.Errorf("the widget is on page %d, wanted page one", w.Page)
	}
	if w.Rect != [4]float64{20, 100, 180, 130} {
		t.Errorf("the widget is at %v", w.Rect)
	}
	if _, ok := f.Field("name"); !ok {
		t.Error("the field cannot be found by its name")
	}
	if _, ok := f.Field("nothing"); ok {
		t.Error("a name nothing has was found")
	}
	if f.HasXFA() {
		t.Error("said it carries XFA when it does not")
	}
}

func TestAFieldsNameIsTheWholePath(t *testing.T) {
	// A field's own name is unique only among its brothers; the name it is
	// addressed by is every part from the root down, with dots between.
	d := formDoc(t, func(w *reader.Writer, page reader.Ref) (reader.Dict, reader.Array) {
		leaf := reader.Dict{"T": str("street"), "Rect": nums(0, 0, 10, 10)}
		middle := reader.Dict{"T": str("home"), "FT": reader.Name("Tx"),
			"Kids": reader.Array{leaf}}
		root := reader.Dict{"T": str("address"), "Kids": reader.Array{middle}}
		return reader.Dict{"Fields": reader.Array{w.Add(root)}}, nil
	})
	f, ok := Read(d)
	if !ok {
		t.Fatal("no form")
	}
	if _, ok := f.Field("address.home.street"); !ok {
		var have []string
		for _, fld := range f.Fields() {
			have = append(have, fld.Name)
		}
		t.Fatalf("wanted address.home.street, read %v", have)
	}
}

func TestWhatAFieldTakesFromItsParents(t *testing.T) {
	// Nearly everything that matters may be said at any level and is taken
	// from the nearest one that says it.
	d := formDoc(t, func(w *reader.Writer, page reader.Ref) (reader.Dict, reader.Array) {
		leaf := reader.Dict{"T": str("leaf"), "Rect": nums(0, 0, 10, 10)}
		root := reader.Dict{
			"T": str("root"), "FT": reader.Name("Tx"), "V": str("inherited"),
			"Ff": reader.Integer(flagMultiline), "MaxLen": reader.Integer(40),
			"Q": reader.Integer(2), "DA": str("/Helv 9 Tf 1 0 0 rg"),
			"Kids": reader.Array{leaf},
		}
		return reader.Dict{"Fields": reader.Array{w.Add(root)}}, nil
	})
	f, _ := Read(d)
	fld, ok := f.Field("root.leaf")
	if !ok {
		t.Fatal("the leaf was not read")
	}
	if fld.Kind != Text || fld.Value != "inherited" || !fld.Multiline ||
		fld.MaxLen != 40 || fld.Quadding != 2 {
		t.Errorf("kind %s value %q multiline %v maxlen %d quadding %d",
			fld.Kind, fld.Value, fld.Multiline, fld.MaxLen, fld.Quadding)
	}
	if !strings.Contains(fld.defaultAppearance, "1 0 0 rg") {
		t.Errorf("the default appearance is %q", fld.defaultAppearance)
	}
}

func TestAFieldWithSeveralPlacesOnThePage(t *testing.T) {
	// Kids without names of their own are not fields but widgets: one field
	// asking the same thing in two places.
	d := formDoc(t, func(w *reader.Writer, page reader.Ref) (reader.Dict, reader.Array) {
		a := w.Add(reader.Dict{"Subtype": reader.Name("Widget"), "Rect": nums(0, 0, 10, 10), "P": page})
		b := w.Add(reader.Dict{"Subtype": reader.Name("Widget"), "Rect": nums(20, 0, 30, 10), "P": page})
		root := w.Add(reader.Dict{"T": str("both"), "FT": reader.Name("Tx"),
			"Kids": reader.Array{a, b}})
		return reader.Dict{"Fields": reader.Array{root}}, reader.Array{a, b}
	})
	f, _ := Read(d)
	if len(f.Fields()) != 1 {
		t.Fatalf("read %d fields, wanted one", len(f.Fields()))
	}
	if n := len(f.Fields()[0].Widgets); n != 2 {
		t.Fatalf("read %d widgets, wanted two", n)
	}
}

func TestWhichSortOfFieldItIs(t *testing.T) {
	// The type says only three things; the flags say which of them it is.
	for _, c := range []struct {
		kind  reader.Name
		flags int64
		want  Kind
		name  string
	}{
		{"Tx", 0, Text, "text"},
		{"Btn", 0, Checkbox, "checkbox"},
		{"Btn", flagRadio, Radio, "radio"},
		{"Btn", flagPushButton, PushButton, "button"},
		{"Btn", flagPushButton | flagRadio, PushButton, "button"},
		{"Ch", 0, ListBox, "list"},
		{"Ch", flagCombo, ComboBox, "combo"},
		{"Sig", 0, Signature, "signature"},
		{"Zz", 0, Text, "text"},
	} {
		if got := kindOf(c.kind, c.flags); got != c.want || got.String() != c.name {
			t.Errorf("%s with flags %d read as %s", c.kind, c.flags, got)
		}
	}
	if got := Kind(99).String(); got != "unknown" {
		t.Errorf("a kind that does not exist is called %q", got)
	}
}

func TestEveryFlagIsRead(t *testing.T) {
	all := int64(flagReadOnly | flagRequired | flagNoExport | flagMultiline |
		flagPassword | flagComb | flagSort | flagMultiSelect | flagEdit)
	_, f := textFieldDoc(t, reader.Dict{"Ff": reader.Integer(all), "MaxLen": reader.Integer(9)})
	fld := f.Fields()[0]
	for name, got := range map[string]bool{
		"read-only": fld.ReadOnly, "required": fld.Required, "no-export": fld.NoExport,
		"multiline": fld.Multiline, "password": fld.Password, "comb": fld.Comb,
		"sorted": fld.Sorted, "multi-select": fld.MultiSelect, "editable": fld.Editable,
	} {
		if !got {
			t.Errorf("%s was not read", name)
		}
	}
}

func TestAValueIsReadHoweverItIsWritten(t *testing.T) {
	for _, c := range []struct {
		why   string
		value reader.Object
		want  string
		all   []string
	}{
		{"a string", str("plain"), "plain", nil},
		{"a name, which is what a tick is", reader.Name("Yes"), "Yes", nil},
		{"UTF-16, which is what anything but Latin needs",
			reader.String([]byte{0xFE, 0xFF, 0x00, 0x41, 0x03, 0xA9}), "AΩ", nil},
		{"a surrogate pair, for something outside the basic plane",
			reader.String([]byte{0xFE, 0xFF, 0xD8, 0x3D, 0xDE, 0x00}), "\U0001F600", nil},
		{"a lone high surrogate, which is a file being wrong",
			reader.String([]byte{0xFE, 0xFF, 0xD8, 0x3D}), "�", nil},
		{"several, which is a list box with several rows chosen",
			reader.Array{str("one"), str("two")}, "one", []string{"one", "two"}},
		{"several of which none can be read", reader.Array{reader.Integer(1)}, "", nil},
		{"something that is neither", reader.Integer(3), "", nil},
	} {
		_, f := textFieldDoc(t, reader.Dict{"V": c.value})
		fld := f.Fields()[0]
		if fld.Value != c.want {
			t.Errorf("%s: read %q, wanted %q", c.why, fld.Value, c.want)
		}
		if len(fld.Values) != len(c.all) {
			t.Errorf("%s: read %v, wanted %v", c.why, fld.Values, c.all)
		}
	}
}

func TestTheRowsOfAChoiceField(t *testing.T) {
	// A row is one string that is both what is stored and what is shown, or a
	// pair that says them separately.
	_, f := textFieldDoc(t, reader.Dict{
		"FT": reader.Name("Ch"),
		"Opt": reader.Array{
			str("plain"),
			reader.Array{str("FR"), str("France")},
			reader.Array{str("short")},
			reader.Integer(4),
		},
	})
	got := f.Fields()[0].Options
	want := []Option{{"plain", "plain"}, {"FR", "France"}}
	if len(got) != len(want) {
		t.Fatalf("read %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d is %v, wanted %v", i, got[i], want[i])
		}
	}
	if f.Fields()[0].Text() != "" {
		t.Errorf("an unchosen field shows %q", f.Fields()[0].Text())
	}
}

func TestOptionsThatAreNotAList(t *testing.T) {
	_, f := textFieldDoc(t, reader.Dict{"FT": reader.Name("Ch"), "Opt": reader.Integer(2)})
	if got := f.Fields()[0].Options; got != nil {
		t.Errorf("read %v from something that is not a list", got)
	}
}

func TestARectangleIsPutTheRightWayRound(t *testing.T) {
	for _, c := range []struct {
		why  string
		rect reader.Object
		want [4]float64
		ok   bool
	}{
		{"the usual way", nums(10, 20, 30, 40), [4]float64{10, 20, 30, 40}, true},
		{"corners the other way about", nums(30, 40, 10, 20), [4]float64{10, 20, 30, 40}, true},
		{"too few numbers", nums(1, 2), [4]float64{}, false},
		{"not numbers", reader.Array{str("a"), str("b"), str("c"), str("d")}, [4]float64{}, false},
		{"not a list", reader.Integer(4), [4]float64{}, false},
	} {
		_, f := textFieldDoc(t, reader.Dict{"Rect": c.rect})
		got := f.Fields()[0].Widgets[0].Rect
		if c.ok && got != c.want {
			t.Errorf("%s: read %v, wanted %v", c.why, got, c.want)
		}
		if !c.ok && got != [4]float64{} {
			t.Errorf("%s: read %v, wanted nothing", c.why, got)
		}
	}
}

func TestAWidgetTheDocumentAsksNotToShow(t *testing.T) {
	for _, c := range []struct {
		flags int64
		want  bool
	}{{0, false}, {1 << 1, true}, {1 << 5, true}, {1 << 2, false}} {
		_, f := textFieldDoc(t, reader.Dict{"F": reader.Integer(c.flags)})
		if got := f.Fields()[0].Widgets[0].Hidden; got != c.want {
			t.Errorf("flags %d: hidden is %v", c.flags, got)
		}
	}
}

func TestTheNameAButtonCallsChosen(t *testing.T) {
	// Every checkbox widget names its own "on" state, and a row of radio
	// buttons gives each of its widgets a different one — which is how the
	// field's single value says which button was pressed.
	d := formDoc(t, func(w *reader.Writer, page reader.Ref) (reader.Dict, reader.Array) {
		blank := w.Add(&reader.Stream{Dict: reader.Dict{"BBox": nums(0, 0, 10, 10)}, Raw: []byte("")})
		one := w.Add(reader.Dict{"Subtype": reader.Name("Widget"), "Rect": nums(0, 0, 10, 10), "P": page,
			"AP": reader.Dict{"N": reader.Dict{"Off": blank, "Zurich": blank}}})
		two := w.Add(reader.Dict{"Subtype": reader.Name("Widget"), "Rect": nums(20, 0, 30, 10), "P": page,
			"AP": reader.Dict{"N": reader.Dict{"Off": blank, "Anvers": blank}}})
		field := w.Add(reader.Dict{"T": str("city"), "FT": reader.Name("Btn"),
			"Ff": reader.Integer(flagRadio), "Kids": reader.Array{one, two}})
		return reader.Dict{"Fields": reader.Array{field}}, reader.Array{one, two}
	})
	f, _ := Read(d)
	fld := f.Fields()[0]
	states := fld.States()
	if len(states) != 2 || states[0] != "Zurich" || states[1] != "Anvers" {
		t.Fatalf("the buttons answer to %v", states)
	}
}

func TestAWidgetWithNothingToSayAboutItsStates(t *testing.T) {
	for _, c := range []struct {
		why string
		ap  reader.Object
	}{
		{"no appearance dictionary", nil},
		{"an appearance dictionary with no normal one", reader.Dict{"D": reader.Dict{}}},
		{"a normal appearance with only Off in it", reader.Dict{"N": reader.Dict{"Off": reader.Integer(0)}}},
	} {
		extra := reader.Dict{"FT": reader.Name("Btn")}
		if c.ap != nil {
			extra["AP"] = c.ap
		}
		_, f := textFieldDoc(t, extra)
		if got := f.Fields()[0].Widgets[0].On; got != "" {
			t.Errorf("%s: named %q as its chosen state", c.why, got)
		}
	}
}

func TestAFormThatCarriesXFAAsWell(t *testing.T) {
	// Two thirds of the corpus's forms carry Adobe's XML description beside
	// the standard one. Nothing here reads it; it is worth being able to say
	// that it is there.
	for _, xfa := range []reader.Object{
		reader.Array{str("preamble"), str("<xdp/>")},
		&reader.Stream{Dict: reader.Dict{}, Raw: []byte("<xdp/>")},
	} {
		d := formDoc(t, func(w *reader.Writer, page reader.Ref) (reader.Dict, reader.Array) {
			ref := w.Add(reader.Dict{"FT": reader.Name("Tx"), "T": str("a"),
				"Rect": nums(0, 0, 10, 10)})
			return reader.Dict{"Fields": reader.Array{ref}, "XFA": w.Add(xfa)}, nil
		})
		f, ok := Read(d)
		if !ok || !f.HasXFA() {
			t.Errorf("%T: XFA was not noticed", xfa)
		}
	}
}

func TestAFieldTreeThatGoesOnForEver(t *testing.T) {
	// A tree deeper than anything real is a file playing games, and is
	// followed only so far.
	d := formDoc(t, func(w *reader.Writer, page reader.Ref) (reader.Dict, reader.Array) {
		node := w.Reserve()
		w.Put(node, reader.Dict{"T": str("loop"), "FT": reader.Name("Tx"),
			"Kids": reader.Array{reader.Dict{"T": str("down"), "Kids": reader.Array{node}}}})
		return reader.Dict{"Fields": reader.Array{node}}, nil
	})
	f, ok := Read(d)
	if !ok {
		t.Skip("a document that cannot be opened proves nothing here")
	}
	if len(f.Fields()) > maxFieldDepth {
		t.Errorf("followed the tree %d fields deep", len(f.Fields()))
	}
}

func TestTheFormsOwnAlignmentIsUsedWhenAFieldSaysNothing(t *testing.T) {
	d := formDoc(t, func(w *reader.Writer, page reader.Ref) (reader.Dict, reader.Array) {
		ref := w.Add(reader.Dict{"FT": reader.Name("Tx"), "T": str("a"), "Rect": nums(0, 0, 10, 10)})
		return reader.Dict{"Fields": reader.Array{ref}, "Q": reader.Integer(1)}, nil
	})
	f, _ := Read(d)
	if got := f.Fields()[0].Quadding; got != 1 {
		t.Errorf("the field is quadded %d, wanted the form's own 1", got)
	}
}

func TestADocumentThatCannotBeAskedForItsCatalog(t *testing.T) {
	if _, ok := Read(&reader.Document{}); ok {
		t.Error("a document with no catalog was read as having a form")
	}
}
