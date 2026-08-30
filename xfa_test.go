// Copyright (c) 2026, the go-pdfkit/forms authors
// All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package forms

import (
	"testing"

	"github.com/go-pdfkit/reader"
)

// xfaDoc writes a document whose form carries an XFA package, and which may
// say its pages are only a placeholder for it. formDoc cannot: the flag is on
// the catalogue rather than on the form.
func xfaDoc(t *testing.T, needsRendering bool, xfa func(w *reader.Writer) reader.Object) *reader.Document {
	t.Helper()
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	pageRef := w.Add(reader.Dict{"Type": reader.Name("Page"), "Parent": pagesRef,
		"MediaBox": nums(0, 0, 200, 200),
		"Contents": w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte("")})})
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{pageRef}, "Count": reader.Integer(1)})
	field := w.Add(reader.Dict{"FT": reader.Name("Tx"), "T": str("a"), "Rect": nums(0, 0, 10, 10)})
	form := reader.Dict{"Fields": reader.Array{field}}
	if xfa != nil {
		form["XFA"] = xfa(w)
	}
	catalog := reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef,
		"AcroForm": w.Add(form)}
	if needsRendering {
		catalog["NeedsRendering"] = reader.Bool(true)
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

// packet is a part of an XFA package as a document really carries one.
func packet(w *reader.Writer, body string) reader.Ref {
	return w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte(body)})
}

func TestTheTwoKindsOfXFA(t *testing.T) {
	// This is the difference that matters, and it is not the same question as
	// HasXFA. A static form carries a full standard form beside the XML and
	// everything here works on it; a dynamic one's pages are a panel saying
	// the viewer cannot show the document.
	for _, tc := range []struct {
		name           string
		needsRendering bool
		wantDynamic    bool
	}{
		{"static: the XML is a second copy of a readable form", false, false},
		{"dynamic: the pages are a placeholder", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := xfaDoc(t, tc.needsRendering, func(w *reader.Writer) reader.Object {
				return w.Add(reader.Array{str("template"), packet(w, "<template/>")})
			})
			f, ok := Read(d)
			if !ok {
				t.Fatal("the form was not read")
			}
			if !f.HasXFA() {
				t.Error("the package was not noticed")
			}
			if f.Dynamic() != tc.wantDynamic {
				t.Errorf("Dynamic() = %v", f.Dynamic())
			}
		})
	}
}

func TestADocumentThatSaysItNeedsRenderingWithoutXFA(t *testing.T) {
	// The flag alone is not an XFA form. Reading it as one would tell somebody
	// their document is unshowable when it is merely odd.
	d := xfaDoc(t, true, nil)
	f, ok := Read(d)
	if !ok {
		t.Fatal("the form was not read")
	}
	if f.HasXFA() || f.Dynamic() || len(f.Packets()) != 0 {
		t.Errorf("HasXFA=%v Dynamic=%v packets=%d", f.HasXFA(), f.Dynamic(), len(f.Packets()))
	}
}

func TestThePartsOfThePackageComeBack(t *testing.T) {
	// The form cannot be laid out from here, but the values can be read: what
	// has been filled in lives in the datasets part, as ordinary XML.
	d := xfaDoc(t, true, func(w *reader.Writer) reader.Object {
		return w.Add(reader.Array{
			str("template"), packet(w, "<template><subform/></template>"),
			str("datasets"), packet(w, "<datasets><data><name>Ada</name></data></datasets>"),
		})
	})
	f, ok := Read(d)
	if !ok {
		t.Fatal("the form was not read")
	}
	got := f.Packets()
	if len(got) != 2 {
		t.Fatalf("%d parts, want 2", len(got))
	}
	if got[0].Name != "template" || string(got[0].Data) != "<template><subform/></template>" {
		t.Errorf("the first part is %+v", got[0])
	}
	if got[1].Name != "datasets" || !contains(string(got[1].Data), "Ada") {
		t.Errorf("the second part is %+v", got[1])
	}
}

func TestAPackageThatIsOneStream(t *testing.T) {
	// A package may be a single stream rather than a list of parts, and then
	// it has no name.
	d := xfaDoc(t, false, func(w *reader.Writer) reader.Object {
		return packet(w, "<xdp/>")
	})
	f, ok := Read(d)
	if !ok {
		t.Fatal("the form was not read")
	}
	got := f.Packets()
	if len(got) != 1 || got[0].Name != "" || string(got[0].Data) != "<xdp/>" {
		t.Errorf("got %+v", got)
	}
}

func TestAPackageThatIsNotWhatItSays(t *testing.T) {
	for _, tc := range []struct {
		name  string
		xfa   func(w *reader.Writer) reader.Object
		parts int
	}{
		{"a list whose entries are not streams", func(w *reader.Writer) reader.Object {
			return w.Add(reader.Array{str("template"), str("<template/>")})
		}, 0},
		{"a list that stops on a name", func(w *reader.Writer) reader.Object {
			return w.Add(reader.Array{str("template"), packet(w, "<t/>"), str("datasets")})
		}, 1},
		{"a part still in a filter nothing here unpacks", func(w *reader.Writer) reader.Object {
			return w.Add(reader.Array{str("template"), w.Add(&reader.Stream{
				Dict: reader.Dict{"Filter": reader.Name("NoSuchDecode")}, Raw: []byte("x")})})
		}, 0},
		{"neither a list nor a stream", func(w *reader.Writer) reader.Object {
			return reader.Integer(7)
		}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := xfaDoc(t, false, tc.xfa)
			f, ok := Read(d)
			if !ok {
				t.Fatal("the form was not read")
			}
			if len(f.Packets()) != tc.parts {
				t.Errorf("%d parts, want %d", len(f.Packets()), tc.parts)
			}
		})
	}
}

// contains is here rather than importing strings for one call.
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestAFormWhoseFieldsLiveInTheXML(t *testing.T) {
	// An AcroForm with no fields is usually a leftover, and Read says there is
	// no form — 561 of the 118 833 files in the figure corpus carry an empty
	// one a producer forgot.
	//
	// Unless it carries an XFA package, and then it is the opposite: a form
	// whose fields live in the XML because that is where the whole form lives.
	// Those are exactly the documents a caller most needs told about, and
	// refusing them is what made Dynamic answer for none of the fourteen
	// dynamic forms in a corpus of 2 240.
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	pageRef := w.Add(reader.Dict{"Type": reader.Name("Page"), "Parent": pagesRef,
		"MediaBox": nums(0, 0, 200, 200),
		"Contents": w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte("")})})
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{pageRef}, "Count": reader.Integer(1)})
	form := w.Add(reader.Dict{
		"Fields": reader.Array{},
		"XFA":    w.Add(reader.Array{str("template"), packet(w, "<template/>")}),
	})
	out, err := w.Finish(reader.Dict{"Root": w.Add(reader.Dict{
		"Type": reader.Name("Catalog"), "Pages": pagesRef,
		"AcroForm": form, "NeedsRendering": reader.Bool(true)})})
	if err != nil {
		t.Fatal(err)
	}
	d, err := reader.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	f, ok := Read(d)
	if !ok {
		t.Fatal("a form whose fields are in the XML was called no form at all")
	}
	if len(f.Fields()) != 0 {
		t.Errorf("%d fields came from nowhere", len(f.Fields()))
	}
	if !f.Dynamic() || len(f.Packets()) != 1 {
		t.Errorf("Dynamic=%v packets=%d", f.Dynamic(), len(f.Packets()))
	}
}
