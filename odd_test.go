package forms

import (
	"strings"
	"testing"

	"github.com/go-pdfkit/reader"
)

func TestAValueWiderThanItsBoxStartsAtTheEdge(t *testing.T) {
	// Ranged right or centred, a value too wide to fit would start off the
	// left of the box. It starts at the edge instead, so that its beginning
	// is the part that shows.
	for _, q := range []int64{1, 2} {
		_, f := textFieldDoc(t, reader.Dict{
			"Q": reader.Integer(q), "DA": str("/Helv 12 Tf 0 g"),
			"Rect": nums(0, 0, 30, 16)})
		if err := f.Fields()[0].SetText("far too wide for this"); err != nil {
			t.Fatal(err)
		}
		if got := firstX(t, drawn(t, f)); got < 0 {
			t.Errorf("quadding %d put the text at %v", q, got)
		}
	}
}

func TestACombFieldWithAValueLongerThanItTakes(t *testing.T) {
	// A value read from the file is not cut to the field's length the way one
	// that is typed in is, so the drawing has to stop at the last cell.
	_, f := textFieldDoc(t, reader.Dict{
		"Ff": reader.Integer(flagComb), "MaxLen": reader.Integer(3),
		"V": str("ABCDEFGH"), "Rect": nums(0, 0, 60, 20)})
	got := drawn(t, f)
	if n := strings.Count(got, " Tj"); n != 3 {
		t.Errorf("drew %d cells, wanted three:\n%s", n, got)
	}
}

func TestAFieldOfSeveralLinesStopsAtTheBottomOfItsBox(t *testing.T) {
	_, f := textFieldDoc(t, reader.Dict{
		"Ff": reader.Integer(flagMultiline), "Rect": nums(0, 0, 60, 20),
		"DA": str("/Helv 9 Tf 0 g")})
	if err := f.Fields()[0].SetText(strings.Repeat("Salzburg ", 30)); err != nil {
		t.Fatal(err)
	}
	got := drawn(t, f)
	if n := strings.Count(got, " Tj"); n > 4 {
		t.Errorf("drew %d lines into a box twenty high:\n%s", n, got)
	}
}

func TestSeveralLinesAreRangedAsTheFieldAsks(t *testing.T) {
	var at [3]float64
	for q := 0; q < 3; q++ {
		_, f := textFieldDoc(t, reader.Dict{
			"Ff": reader.Integer(flagMultiline), "Q": reader.Integer(int64(q)),
			"DA": str("/Helv 8 Tf 0 g"), "Rect": nums(0, 0, 150, 60)})
		if err := f.Fields()[0].SetText("short line"); err != nil {
			t.Fatal(err)
		}
		at[q] = firstX(t, drawn(t, f))
	}
	if !(at[0] < at[1] && at[1] < at[2]) {
		t.Errorf("left %v centred %v right %v", at[0], at[1], at[2])
	}
}

func TestSeveralLinesTooWideStartAtTheEdge(t *testing.T) {
	_, f := textFieldDoc(t, reader.Dict{
		"Ff": reader.Integer(flagMultiline), "Q": reader.Integer(2),
		"DA": str("/Helv 12 Tf 0 g"), "Rect": nums(0, 0, 24, 40)})
	if err := f.Fields()[0].SetText("unbreakablylongword"); err != nil {
		t.Fatal(err)
	}
	if got := firstX(t, drawn(t, f)); got < 0 {
		t.Errorf("put the text at %v", got)
	}
}

func TestAListBoxStopsAtTheBottomOfItsBox(t *testing.T) {
	rows := make(reader.Array, 0, 40)
	for i := 0; i < 40; i++ {
		rows = append(rows, str(string(rune('a'+i%26))))
	}
	_, f := textFieldDoc(t, reader.Dict{"FT": reader.Name("Ch"), "Opt": rows,
		"DA": str("/Helv 9 Tf 0 g"), "Rect": nums(0, 0, 60, 30)})
	got := drawn(t, f)
	if n := strings.Count(got, " Tj"); n > 5 {
		t.Errorf("drew %d rows into a box thirty high", n)
	}
}

func TestWhatAChoiceFieldHasChosen(t *testing.T) {
	f := choiceDoc(t, flagCombo)
	if got := f.Fields()[0].selected(); got != nil {
		t.Errorf("an unchosen field has chosen %v", got)
	}
}

func TestASizeThatFitsIsKeptWithinReason(t *testing.T) {
	// A box too small for any readable size still gets one that can be drawn,
	// and a box large enough for anything does not get text a foot high.
	_, tiny := textFieldDoc(t, reader.Dict{"Rect": nums(0, 0, 5, 4)})
	if err := tiny.Fields()[0].SetText("something long"); err != nil {
		t.Fatal(err)
	}
	if got := firstSize(t, drawn(t, tiny)); got != 1 {
		t.Errorf("a box five wide set its text at %v", got)
	}

	_, huge := textFieldDoc(t, reader.Dict{"Rect": nums(0, 0, 4000, 4000)})
	if err := huge.Fields()[0].SetText("x"); err != nil {
		t.Fatal(err)
	}
	if got := firstSize(t, drawn(t, huge)); got != 144 {
		t.Errorf("a very large box set its text at %v", got)
	}
}

func TestThingsInTheFieldTreeThatAreNotFields(t *testing.T) {
	// A file may put anything in a list, and a kid that is not a dictionary
	// is neither a field nor a widget.
	d := formDoc(t, func(w *reader.Writer, page reader.Ref) (reader.Dict, reader.Array) {
		root := w.Add(reader.Dict{"T": str("root"), "FT": reader.Name("Tx"),
			"Kids": reader.Array{reader.Integer(3), str("nonsense")}})
		return reader.Dict{"Fields": reader.Array{root}}, nil
	})
	f, ok := Read(d)
	if !ok {
		t.Fatal("no form")
	}
	if n := len(f.Fields()[0].Widgets); n != 0 {
		t.Errorf("made %d widgets out of things that are not dictionaries", n)
	}
}

func TestAPageThatCannotBeRead(t *testing.T) {
	// A page tree that says it has more pages than it holds is a file being
	// wrong, and the widgets whose page cannot be found simply have none.
	d := formDoc(t, func(w *reader.Writer, page reader.Ref) (reader.Dict, reader.Array) {
		ref := w.Add(reader.Dict{"FT": reader.Name("Tx"), "T": str("a"),
			"Rect": nums(0, 0, 10, 10), "Subtype": reader.Name("Widget")})
		return reader.Dict{"Fields": reader.Array{ref}}, reader.Array{ref}
	})
	f, _ := Read(d)
	if got := f.Fields()[0].Widgets[0].Page; got != 1 {
		t.Errorf("the widget says it is on page %d", got)
	}
}

func TestFillingAButtonByTheNameOfItsState(t *testing.T) {
	// A group of more than two buttons is not yes-or-no: it is pressed by
	// naming which one.
	d := formDoc(t, func(w *reader.Writer, page reader.Ref) (reader.Dict, reader.Array) {
		blank := w.Add(&reader.Stream{Dict: reader.Dict{"BBox": nums(0, 0, 10, 10)}, Raw: []byte("")})
		one := w.Add(reader.Dict{"Subtype": reader.Name("Widget"), "Rect": nums(0, 0, 10, 10), "P": page,
			"AP": reader.Dict{"N": reader.Dict{"Off": blank, "Zurich": blank}}})
		field := w.Add(reader.Dict{"T": str("city"), "FT": reader.Name("Btn"),
			"Ff": reader.Integer(flagRadio), "Kids": reader.Array{one}})
		return reader.Dict{"Fields": reader.Array{field}}, reader.Array{one}
	})
	f, _ := Read(d)
	if err := f.Fill("city", "Zurich"); err != nil {
		t.Fatal(err)
	}
	if got := f.Fields()[0].Value; got != "Zurich" {
		t.Errorf("holds %q", got)
	}
}

func TestMeasuringWithTheDocumentsOwnWidths(t *testing.T) {
	// A font that says how wide its characters are is believed, since it is
	// the one that will be used.
	d := formDoc(t, func(w *reader.Writer, page reader.Ref) (reader.Dict, reader.Array) {
		widths := make(reader.Array, 0, 96)
		for i := 0; i < 96; i++ {
			widths = append(widths, reader.Integer(700))
		}
		font := w.Add(reader.Dict{"Type": reader.Name("Font"), "Subtype": reader.Name("Type1"),
			"BaseFont": reader.Name("Helvetica"), "Encoding": reader.Name("WinAnsiEncoding"),
			"FirstChar": reader.Integer(32), "LastChar": reader.Integer(127), "Widths": widths})
		ref := w.Add(reader.Dict{"FT": reader.Name("Tx"), "T": str("a"),
			"Rect": nums(0, 0, 200, 20), "V": str("iii"),
			"Subtype": reader.Name("Widget"), "P": page})
		return reader.Dict{"Fields": reader.Array{ref}, "DA": str("/Wide 10 Tf 0 g"),
			"DR": reader.Dict{"Font": reader.Dict{"Wide": font}}}, reader.Array{ref}
	})
	f, _ := Read(d)
	m := f.measurer("Wide")
	if got := m.width("iii"); got < 2.0 {
		t.Errorf("three characters seven tenths of an em wide measured %v", got)
	}
}

func TestMeasuringSomethingNothingCanMeasure(t *testing.T) {
	// A face that would not parse leaves nothing to measure with, and a
	// character no face has is no better. Half an em keeps a value inside
	// its box either way.
	m := &metrics{code: map[rune]byte{}}
	if got := m.width("ab"); got != 1 {
		t.Errorf("two characters with nothing to measure them came to %v", got)
	}
	if a, d := m.height(); a != 0.75 || d != 0.25 {
		t.Errorf("a face with no metrics is %v high and %v deep", a, d)
	}
	real := (&Form{}).measurerFor(sansStandIn)
	if got := real.runeWidth('\U000E0100'); got != 0.5 {
		t.Errorf("a character the face has not measured %v", got)
	}
}

func TestChoosingAFaceByName(t *testing.T) {
	for _, c := range []struct {
		name string
		want *standIn
	}{
		{"Helv", sansStandIn},
		{"Cour", monoStandIn},
		{"CoBo", monoStandIn},
		{"Courier-Bold", monoStandIn},
		{"SomethingMono", monoStandIn},
		{"TiRo", serifStandIn},
		{"Times-Roman", serifStandIn},
		{"MySerif", serifStandIn},
		{"MySansSerif", sansStandIn},
		{"", sansStandIn},
	} {
		if got := standInFor(c.name); got != c.want {
			t.Errorf("%q chose the wrong face", c.name)
		}
	}
}

func TestAFontNameThatNamesNothing(t *testing.T) {
	_, f := textFieldDoc(t, nil)
	if _, ok := f.fontDict(""); ok {
		t.Error("an empty name found a font")
	}
	if _, ok := f.fontDict("Nowhere"); ok {
		t.Error("a name nothing has found a font")
	}
	bare := &Form{}
	if _, ok := bare.fontDict("Helv"); ok {
		t.Error("a form with no resources found a font")
	}
}

func TestAFontWhoseCodesStandForNothing(t *testing.T) {
	// A font with no encoding says nothing about what its codes mean, and a
	// value is then written as the eight-bit thing every such font is.
	d := formDoc(t, func(w *reader.Writer, page reader.Ref) (reader.Dict, reader.Array) {
		font := w.Add(reader.Dict{"Type": reader.Name("Font"),
			"Subtype": reader.Name("Type0"), "BaseFont": reader.Name("Odd")})
		ref := w.Add(reader.Dict{"FT": reader.Name("Tx"), "T": str("a"),
			"Rect": nums(0, 0, 100, 20), "V": str("hello"),
			"Subtype": reader.Name("Widget"), "P": page})
		return reader.Dict{"Fields": reader.Array{ref}, "DA": str("/Odd 9 Tf 0 g"),
			"DR": reader.Dict{"Font": reader.Dict{"Odd": font}}}, reader.Array{ref}
	})
	f, _ := Read(d)
	fld := f.Fields()[0]
	app, ok := fld.Appearance(fld.Widgets[0])
	if !ok {
		t.Fatal("drew nothing")
	}
	if !strings.Contains(string(app.Content), "Tj") {
		t.Errorf("drew no text at all:\n%s", app.Content)
	}
}

func TestAPageTreeThatPromisesMoreThanItHolds(t *testing.T) {
	// A page tree whose count is larger than its list is a file being wrong.
	// The pages that are there are still walked for their widgets.
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	pageRef := w.Reserve()
	field := w.Add(reader.Dict{"FT": reader.Name("Tx"), "T": str("a"),
		"Rect": nums(0, 0, 10, 10), "Subtype": reader.Name("Widget"), "P": pageRef})
	w.Put(pageRef, reader.Dict{"Type": reader.Name("Page"), "Parent": pagesRef,
		"MediaBox": nums(0, 0, 200, 200), "Annots": reader.Array{field}})
	// The second kid is not a page at all, which is a file being wrong in a
	// way that must not stop the first one being read.
	broken := w.Add(reader.Integer(7))
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{pageRef, broken}, "Count": reader.Integer(2)})
	catalog := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef,
		"AcroForm": w.Add(reader.Dict{"Fields": reader.Array{field}})})
	out, err := w.Finish(reader.Dict{"Root": catalog})
	if err != nil {
		t.Fatal(err)
	}
	d, err := reader.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	f, ok := Read(d)
	if !ok {
		t.Fatal("no form")
	}
	if got := f.Fields()[0].Widgets[0].Page; got != 1 {
		t.Errorf("the widget says it is on page %d", got)
	}
}

func TestAFontWhoseCodeStandsForMoreThanOneCharacter(t *testing.T) {
	// A ligature is one code and several characters, and there is no one
	// character to write it back as, so it takes part in nothing.
	d := formDoc(t, func(w *reader.Writer, page reader.Ref) (reader.Dict, reader.Array) {
		toUnicode := w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte(
			"/CIDInit /ProcSet findresource begin 12 dict begin begincmap\n" +
				"1 begincodespacerange <00> <ff> endcodespacerange\n" +
				"1 beginbfchar <41> <0066006600690000> endbfchar\n" +
				"endcmap end end")})
		font := w.Add(reader.Dict{"Type": reader.Name("Font"), "Subtype": reader.Name("Type1"),
			"BaseFont": reader.Name("Helvetica"), "ToUnicode": toUnicode})
		ref := w.Add(reader.Dict{"FT": reader.Name("Tx"), "T": str("a"),
			"Rect": nums(0, 0, 100, 20), "V": str("x"),
			"Subtype": reader.Name("Widget"), "P": page})
		return reader.Dict{"Fields": reader.Array{ref}, "DA": str("/Lig 9 Tf 0 g"),
			"DR": reader.Dict{"Font": reader.Dict{"Lig": font}}}, reader.Array{ref}
	})
	f, _ := Read(d)
	fld := f.Fields()[0]
	if _, ok := fld.Appearance(fld.Widgets[0]); !ok {
		t.Error("drew nothing")
	}
}

func TestAResourceDictionaryWithNoFontsInIt(t *testing.T) {
	d := formDoc(t, func(w *reader.Writer, page reader.Ref) (reader.Dict, reader.Array) {
		ref := w.Add(reader.Dict{"FT": reader.Name("Tx"), "T": str("a"),
			"Rect": nums(0, 0, 100, 20), "V": str("x"),
			"Subtype": reader.Name("Widget"), "P": page})
		return reader.Dict{"Fields": reader.Array{ref}, "DA": str("/Helv 9 Tf 0 g"),
			"DR": reader.Dict{"XObject": reader.Dict{}}}, reader.Array{ref}
	})
	f, _ := Read(d)
	if _, ok := f.fontDict("Helv"); ok {
		t.Error("found a font in a resource dictionary that has none")
	}
	fld := f.Fields()[0]
	if _, ok := fld.Appearance(fld.Widgets[0]); !ok {
		t.Error("drew nothing")
	}
}
