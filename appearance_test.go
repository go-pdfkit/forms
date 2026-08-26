package forms

import (
	"strings"
	"testing"

	"github.com/go-pdfkit/reader"
)

// drawn is the appearance a field's one widget gets, as a string, which is
// what these tests read.
func drawn(t *testing.T, f *Form) string {
	t.Helper()
	fld := f.Fields()[0]
	app, ok := fld.Appearance(fld.Widgets[0])
	if !ok {
		t.Fatal("the field would not draw an appearance")
	}
	if app.BBox[2] <= 0 || app.BBox[3] <= 0 {
		t.Fatalf("the box it drew in is %v", app.BBox)
	}
	return string(app.Content)
}

func TestAValueIsDrawnIntoItsBox(t *testing.T) {
	_, f := textFieldDoc(t, nil)
	if err := f.Fields()[0].SetText("Mozart"); err != nil {
		t.Fatal(err)
	}
	got := drawn(t, f)
	for _, want := range []string{"/Tx BMC", "q\n", "re W n", "BT", "/Helv ", "Tf", "(Mozart) Tj", "ET", "Q\nEMC"} {
		if !strings.Contains(got, want) {
			t.Errorf("the appearance has no %q in it:\n%s", want, got)
		}
	}
}

func TestTheColourOfTheDefaultAppearanceIsKept(t *testing.T) {
	// A colour is written a dozen ways, so what is not the font and the size
	// is copied over as it stands rather than read.
	_, f := textFieldDoc(t, reader.Dict{"DA": str("0 0.2 0.6 rg /Helv 9 Tf 1 Tr")})
	if err := f.Fields()[0].SetText("x"); err != nil {
		t.Fatal(err)
	}
	got := drawn(t, f)
	if !strings.Contains(got, "0 0.2 0.6 rg") || !strings.Contains(got, "1 Tr") {
		t.Errorf("the appearance lost what the default said:\n%s", got)
	}
	if !strings.Contains(got, "/Helv 9 Tf") {
		t.Errorf("the size it was told to use is not there:\n%s", got)
	}
}

func TestAValueIsRangedAsTheFieldAsks(t *testing.T) {
	// Ranged left, centred and ranged right put the same text at three
	// different places, and each further along than the last.
	var at [3]float64
	for q := 0; q < 3; q++ {
		_, f := textFieldDoc(t, reader.Dict{"Q": reader.Integer(int64(q)),
			"DA": str("/Helv 10 Tf 0 g")})
		if err := f.Fields()[0].SetText("short"); err != nil {
			t.Fatal(err)
		}
		at[q] = firstX(t, drawn(t, f))
	}
	if !(at[0] < at[1] && at[1] < at[2]) {
		t.Errorf("ranged left at %v, centred at %v, ranged right at %v", at[0], at[1], at[2])
	}
}

// firstX reads the x of the first text matrix in an appearance.
func firstX(t *testing.T, s string) float64 {
	t.Helper()
	for _, line := range strings.Split(s, "\n") {
		if strings.HasSuffix(line, " Tm") {
			fields := strings.Fields(line)
			var v float64
			if _, err := fmtSscan(fields[4], &v); err != nil {
				t.Fatalf("could not read %q", line)
			}
			return v
		}
	}
	t.Fatalf("no text matrix in:\n%s", s)
	return 0
}

func TestTextTooWideForItsBoxIsMadeToFit(t *testing.T) {
	// A size of zero is what nearly every form asks for, and means as large
	// as fits. A long value must come out smaller than a short one.
	sizeOf := func(text string) float64 {
		_, f := textFieldDoc(t, nil)
		if err := f.Fields()[0].SetText(text); err != nil {
			t.Fatal(err)
		}
		return firstSize(t, drawn(t, f))
	}
	short, long := sizeOf("ab"), sizeOf(strings.Repeat("Wolfgang Amadeus ", 8))
	if !(long < short) {
		t.Errorf("a long value was set at %v and a short one at %v", long, short)
	}
	if long < 1 {
		t.Errorf("a very long value was set at %v, which nothing could draw", long)
	}
}

// firstSize reads the size out of the first Tf in an appearance.
func firstSize(t *testing.T, s string) float64 {
	t.Helper()
	for _, line := range strings.Split(s, "\n") {
		if strings.HasSuffix(line, " Tf") {
			fields := strings.Fields(line)
			var v float64
			if _, err := fmtSscan(fields[1], &v); err != nil {
				t.Fatalf("could not read %q", line)
			}
			return v
		}
	}
	t.Fatalf("no font size in:\n%s", s)
	return 0
}

func TestAFieldOfSeveralLinesBreaksItsValue(t *testing.T) {
	_, f := textFieldDoc(t, reader.Dict{
		"Ff": reader.Integer(flagMultiline), "Rect": nums(0, 0, 120, 90),
		"DA": str("/Helv 8 Tf 0 g")})
	if err := f.Fields()[0].SetText("Wolfgang Amadeus Mozart was born in Salzburg\n\nin 1756"); err != nil {
		t.Fatal(err)
	}
	got := drawn(t, f)
	if n := strings.Count(got, " Tj"); n < 3 {
		t.Errorf("broke the value into %d lines:\n%s", n, got)
	}
	if !strings.Contains(got, "(in 1756) Tj") {
		t.Errorf("the newline somebody typed was not kept:\n%s", got)
	}
}

func TestAFieldOfSeveralLinesSizesItselfToFit(t *testing.T) {
	_, f := textFieldDoc(t, reader.Dict{
		"Ff": reader.Integer(flagMultiline), "Rect": nums(0, 0, 100, 40)})
	if err := f.Fields()[0].SetText(strings.Repeat("Salzburg ", 40)); err != nil {
		t.Fatal(err)
	}
	if got := firstSize(t, drawn(t, f)); got > 12 || got < 4 {
		t.Errorf("set at %v", got)
	}
}

func TestACombFieldPutsOneCharacterInEachCell(t *testing.T) {
	// A comb field is divided into as many equal cells as it takes
	// characters, which is how a form asks for a code a digit at a time.
	_, f := textFieldDoc(t, reader.Dict{
		"Ff": reader.Integer(flagComb), "MaxLen": reader.Integer(5),
		"Rect": nums(0, 0, 100, 20)})
	if err := f.Fields()[0].SetText("ABCDEFG"); err != nil {
		t.Fatal(err)
	}
	got := drawn(t, f)
	if n := strings.Count(got, " Tj"); n != 5 {
		t.Errorf("drew %d characters, wanted the five it takes:\n%s", n, got)
	}
	for _, want := range []string{"(A) Tj", "(E) Tj"} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "(F) Tj") {
		t.Errorf("drew past the end of the comb:\n%s", got)
	}
}

func TestAListBoxDrawsItsRowsAndMarksTheChosenOnes(t *testing.T) {
	f := choiceDoc(t, flagMultiSelect)
	fld := f.Fields()[0]
	if err := fld.Choose("BE"); err != nil {
		t.Fatal(err)
	}
	got := drawn(t, f)
	if !strings.Contains(got, "(France) Tj") || !strings.Contains(got, "(Belgique) Tj") {
		t.Errorf("did not draw both rows:\n%s", got)
	}
	if !strings.Contains(got, "re f") {
		t.Errorf("did not mark the chosen row:\n%s", got)
	}
}

func TestSomeFieldsHaveNoAppearanceToDraw(t *testing.T) {
	// A checkbox already carries a drawing for each of its states, so
	// choosing among them is a matter of saying which, not of drawing one.
	// A push button and a signature field are not filled in at all.
	for _, kind := range []reader.Name{"Btn", "Sig"} {
		_, f := textFieldDoc(t, reader.Dict{"FT": kind})
		fld := f.Fields()[0]
		if _, ok := fld.Appearance(fld.Widgets[0]); ok {
			t.Errorf("%s: drew an appearance it does not need", kind)
		}
	}
}

func TestAWidgetOfNoSizeDrawsNothing(t *testing.T) {
	for _, rect := range []reader.Array{nums(10, 10, 10, 40), nums(10, 10, 40, 10)} {
		_, f := textFieldDoc(t, reader.Dict{"Rect": rect})
		fld := f.Fields()[0]
		if err := fld.SetText("x"); err != nil {
			t.Fatal(err)
		}
		if _, ok := fld.Appearance(fld.Widgets[0]); ok {
			t.Errorf("%v: drew into a box of no size", rect)
		}
	}
}

func TestAFieldWhoseFontTheDocumentDoesNotCarry(t *testing.T) {
	// One of the corpus's files names HelveticaLTStd-Bold on 106 of its
	// fields and does not carry it. A stream naming a font nothing can find
	// draws nothing at all, so the form's own fallback is used instead.
	_, f := textFieldDoc(t, reader.Dict{"DA": str("/Nowhere 9 Tf 0 g")})
	fld := f.Fields()[0]
	if err := fld.SetText("x"); err != nil {
		t.Fatal(err)
	}
	app, ok := fld.Appearance(fld.Widgets[0])
	if !ok {
		t.Fatal("drew nothing")
	}
	if app.FontName != "Helv" || app.Font == nil {
		t.Errorf("named %q and found %v", app.FontName, app.Font != nil)
	}
	if !strings.Contains(string(app.Content), "/Helv ") {
		t.Errorf("the stream still names the missing font:\n%s", app.Content)
	}
}

func TestAFormWithNoResourcesAtAll(t *testing.T) {
	// Then there is no font to find, and the writer is left to supply a
	// standard one under the name.
	d := formDoc(t, func(w *reader.Writer, page reader.Ref) (reader.Dict, reader.Array) {
		ref := w.Add(reader.Dict{"FT": reader.Name("Tx"), "T": str("a"),
			"Rect": nums(0, 0, 80, 20), "V": str("x"),
			"Subtype": reader.Name("Widget"), "P": page})
		return reader.Dict{"Fields": reader.Array{ref}}, reader.Array{ref}
	})
	f, _ := Read(d)
	fld := f.Fields()[0]
	app, ok := fld.Appearance(fld.Widgets[0])
	if !ok {
		t.Fatal("drew nothing")
	}
	if app.Font != nil || app.FontName != "Helv" {
		t.Errorf("named %q and found a font: %v", app.FontName, app.Font != nil)
	}
	if !strings.Contains(string(app.Content), "(x) Tj") {
		t.Errorf("drew nothing readable:\n%s", app.Content)
	}
}

func TestTheBorderIsLeftClearOfTheText(t *testing.T) {
	wide, _ := textFieldDoc(t, reader.Dict{"BS": reader.Dict{"W": reader.Integer(5)}})
	_ = wide
	_, f := textFieldDoc(t, reader.Dict{"BS": reader.Dict{"W": reader.Integer(5)},
		"DA": str("/Helv 8 Tf 0 g")})
	if err := f.Fields()[0].SetText("x"); err != nil {
		t.Fatal(err)
	}
	if got := firstX(t, drawn(t, f)); got < 5 {
		t.Errorf("the text starts %v from the edge, inside a border five wide", got)
	}
}

func TestReadingADefaultAppearance(t *testing.T) {
	for _, c := range []struct {
		da   string
		name string
		size float64
		rest string
	}{
		{"/Helv 0 Tf 0 g", "Helv", 0, "0 g"},
		{"/TiRo 12 Tf 1 0 0 rg", "TiRo", 12, "1 0 0 rg"},
		{"0 g /Helv 9 Tf", "Helv", 9, "0 g"},
		{"", "Helv", 0, ""},
		{"0 g", "Helv", 0, "0 g"},
		{"Tf", "Helv", 0, "Tf"},
		{"/Helv Tf", "Helv", 0, "/Helv Tf"},
		{"/Helv notanumber Tf 0 g", "Helv", 0, "0 g"},
	} {
		name, size, rest := splitDA(c.da)
		if name != c.name || size != c.size || rest != c.rest {
			t.Errorf("%q read as %q %v %q, wanted %q %v %q",
				c.da, name, size, rest, c.name, c.size, c.rest)
		}
	}
}

func TestWritingAStringIntoAContentStream(t *testing.T) {
	got := string(literal([]byte("a(b)c\\d\re\nf")))
	want := `(a\(b\)c\\d\re\nf)`
	if got != want {
		t.Errorf("wrote %s, wanted %s", got, want)
	}
}

func TestWritingANumber(t *testing.T) {
	for _, c := range []struct {
		v    float64
		want string
	}{
		{0, "0"}, {1, "1"}, {1.5, "1.5"}, {-2.25, "-2.25"},
		{1.0 / 3, "0.3333"}, {0.00001, "0"}, {-0.00001, "0"},
	} {
		if got := number(c.v); got != c.want {
			t.Errorf("%v wrote as %q, wanted %q", c.v, got, c.want)
		}
	}
}
