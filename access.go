package forms

import "github.com/go-pdfkit/reader"

// A form is read here and written somewhere else: this package works out what
// a field holds and what its appearance should look like, and the verb layer
// puts the result back in the document. So it has to be possible to say where
// in the document each piece came from, which is what these are for.

// Ref is where the field's own dictionary lives in the document. A field
// written inside another object rather than as one of its own has none, which
// happens in files that put a whole form in a single array.
func (f *Field) Ref() (reader.Ref, bool) { return f.ref, f.ref != reader.Ref{} }

// Dict is the field's own dictionary, as the document holds it. Whoever writes
// the document out copies this and changes what has to change, rather than
// building a field from nothing and losing everything this package does not
// know about — a field's actions, its appearance characteristics, the border
// somebody chose.
func (f *Field) Dict() reader.Dict { return f.dict }

// Ref is where a widget's dictionary lives. For the common field — one that
// shows in exactly one place — this is the same object as the field's own,
// since a file merges the two.
func (w Widget) Ref() (reader.Ref, bool) { return w.ref, w.ref != reader.Ref{} }

// Dict is the widget's own dictionary.
func (w Widget) Dict() reader.Dict { return w.dict }

// Dict is the form's own dictionary — the AcroForm — which holds the defaults
// and the resources every field falls back on.
func (f *Form) Dict() reader.Dict { return f.dict }

// Resources is the dictionary a default appearance's font is named in, or nil
// when the form has none.
func (f *Form) Resources() reader.Dict { return f.resources }

// NeedAppearances is a document saying that the values in it are right and the
// drawings beside them are not, and asking whoever opens it to draw them
// again. Fifteen of the corpus's forms say it. A writer that fills a field and
// draws its appearance should clear it, since it has just done what was asked.
func (f *Form) NeedAppearances() bool {
	v, _ := reader.ToBool(resolve(f.doc, f.dict.Get("NeedAppearances")))
	return v
}

// DefaultAppearance is the form's own, which a field with none of its own
// falls back on.
func (f *Form) DefaultAppearance() string { return f.defaultAppearance }
