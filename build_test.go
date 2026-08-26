package forms

import (
	"testing"

	"github.com/go-pdfkit/reader"
)

// formDoc builds a one-page document whose AcroForm is what the test wants.
// The builder is handed the writer and the page's reference, so that a widget
// can be put on the page and the page can point back at it.
func formDoc(t *testing.T, build func(w *reader.Writer, page reader.Ref) (form reader.Dict, annots reader.Array)) *reader.Document {
	t.Helper()
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	pageRef := w.Reserve()
	form, annots := build(w, pageRef)
	page := reader.Dict{
		"Type": reader.Name("Page"), "Parent": pagesRef,
		"MediaBox": nums(0, 0, 200, 200),
		"Contents": w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte("")}),
	}
	if len(annots) > 0 {
		page["Annots"] = annots
	}
	w.Put(pageRef, page)
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{pageRef}, "Count": reader.Integer(1)})
	catalog := reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef}
	if form != nil {
		catalog["AcroForm"] = w.Add(form)
	}
	out, err := w.Finish(reader.Dict{"Root": w.Add(catalog)})
	if err != nil {
		t.Fatal(err)
	}
	d, err := reader.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// nums writes a row of numbers, which a rectangle and a matrix both are.
func nums(vs ...float64) reader.Array {
	out := make(reader.Array, len(vs))
	for i, v := range vs {
		out[i] = reader.Real(v)
	}
	return out
}

// str writes a text string.
func str(s string) reader.String { return reader.String(s) }

// helv is a resource dictionary holding the one font a form's default
// appearance nearly always names.
func helv(w *reader.Writer) reader.Dict {
	return reader.Dict{"Font": reader.Dict{
		"Helv": w.Add(reader.Dict{"Type": reader.Name("Font"),
			"Subtype": reader.Name("Type1"), "BaseFont": reader.Name("Helvetica"),
			"Encoding": reader.Name("WinAnsiEncoding")}),
	}}
}

// textFieldDoc is the shape most of these tests want: one text field, one
// widget, on the page, with whatever extra entries the test asks for.
func textFieldDoc(t *testing.T, extra reader.Dict) (*reader.Document, *Form) {
	t.Helper()
	d := formDoc(t, func(w *reader.Writer, page reader.Ref) (reader.Dict, reader.Array) {
		field := reader.Dict{
			"FT": reader.Name("Tx"), "T": str("name"),
			"Type": reader.Name("Annot"), "Subtype": reader.Name("Widget"),
			"Rect": nums(20, 100, 180, 130), "P": page,
		}
		for k, v := range extra {
			field[k] = v
		}
		ref := w.Add(field)
		return reader.Dict{
			"Fields": reader.Array{ref},
			"DA":     str("/Helv 0 Tf 0 g"),
			"DR":     helv(w),
		}, reader.Array{ref}
	})
	f, ok := Read(d)
	if !ok {
		t.Fatal("the document was written with a form and read without one")
	}
	return d, f
}
