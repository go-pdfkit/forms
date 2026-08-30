package forms

import (
	"strings"

	"github.com/go-pdfkit/reader"
)

// A Form is a document's fillable part.
type Form struct {
	doc *reader.Document
	// dict is the document's AcroForm dictionary, which holds the defaults
	// every field falls back on.
	dict reader.Dict
	// fields are the ones that can hold a value, in the order the document
	// lists them, with the parents that only group them left out.
	fields []*Field
	byName map[string]*Field
	// defaultAppearance and resources are what a field without its own falls
	// back on when its appearance has to be drawn.
	defaultAppearance string
	resources         reader.Dict
	// quadding is the alignment the whole form asks for, when it asks.
	quadding int
	// dynamic says the pages are a placeholder and the form exists only as
	// XML, and packets are that XML. See xfa.go.
	dynamic bool
	packets []Packet
	// hasXFA says the document carries Adobe's XML form description as well.
	// Nothing here reads it; it is worth being able to say so.
	hasXFA bool
}

// A Kind says what sort of a field it is, which decides both what a value
// means and what its appearance looks like.
type Kind uint8

// The kinds a field may be. A push button is a field that holds no value at
// all — it is a place to click — and a signature field is one this package
// will read and will not pretend to fill.
const (
	Text Kind = iota
	Checkbox
	Radio
	PushButton
	ComboBox
	ListBox
	Signature
)

// String names the kind, for a message someone has to read.
func (k Kind) String() string {
	switch k {
	case Text:
		return "text"
	case Checkbox:
		return "checkbox"
	case Radio:
		return "radio"
	case PushButton:
		return "button"
	case ComboBox:
		return "combo"
	case ListBox:
		return "list"
	case Signature:
		return "signature"
	}
	return "unknown"
}

// A Field is one thing a person fills in.
type Field struct {
	// Name is the whole name, with a dot between each part, which is how a
	// field is addressed from outside: the document's own names are only
	// unique among their brothers.
	Name string
	Kind Kind

	// Value is what the field holds. For a checkbox or a radio button it is
	// the name of the chosen state, and Off means not chosen.
	Value string
	// Values is what a list box holds when more than one row is chosen.
	Values []string
	// Options are the rows of a choice field: what is stored and what is
	// shown, which a file may say separately.
	Options []Option

	// MaxLen is how many characters a text field takes, or zero for as many
	// as one likes.
	MaxLen int
	// Quadding is 0 for ranged left, 1 for centred and 2 for ranged right.
	Quadding int

	ReadOnly bool
	Required bool
	NoExport bool
	// Multiline, Password and Comb are the ways a text field can be unusual.
	// A comb field is one divided into MaxLen equal cells, one character to
	// each, which is how a form asks for a code or a number one digit at a
	// time.
	Multiline bool
	Password  bool
	Comb      bool
	// Sorted and MultiSelect belong to a choice field, and Editable says a
	// combo box will take something that is not one of its rows.
	Sorted      bool
	MultiSelect bool
	Editable    bool

	// Widgets are the places on the pages where the field shows. Nearly every
	// field has one; a field asking the same thing on several pages has more.
	Widgets []Widget

	// defaultAppearance is the field's own, when it has one, and the form's
	// otherwise.
	defaultAppearance string

	form *Form
	dict reader.Dict
	ref  reader.Ref
	// changed says a value was set here rather than read from the file, so
	// that whoever writes the document out knows what has to be written.
	changed bool
}

// An Option is one row of a choice field.
type Option struct {
	// Value is what is stored when the row is chosen and Label what is shown.
	// A file that gives only one gives the value, and the label is the same.
	Value string
	Label string
}

// A Widget is one place on a page where a field shows.
type Widget struct {
	// Page counts from one. It is zero when the document does not say which
	// page the widget is on, which happens in files whose page tree and
	// annotation lists disagree.
	Page int
	// Rect is where on that page, in the page's own coordinates, lower left
	// corner first.
	Rect [4]float64
	// On is the state name that means "chosen", for a checkbox or a radio
	// button. Every such widget has one of its own, which is how a set of
	// radio buttons sharing a value tells which one was pressed.
	On string
	// Hidden says the document asks for this one not to be drawn.
	Hidden bool

	dict reader.Dict
	ref  reader.Ref
}

// maxFieldDepth is how far down a field tree this will go. A tree deeper than
// this is a file playing games rather than a form.
const maxFieldDepth = 32

// Read reads a document's form, or reports that it has none. A document with
// an AcroForm dictionary and no fields in it has none: producers leave the
// dictionary behind, and 561 of the 118 833 files in the figure corpus have
// exactly that and nothing else.
func Read(d *reader.Document) (*Form, bool) {
	catalog, err := d.Catalog()
	if err != nil {
		return nil, false
	}
	dict, ok := d.GetDict(catalog, "AcroForm")
	if !ok {
		return nil, false
	}
	f := &Form{doc: d, dict: dict, byName: map[string]*Field{}}
	if s, ok := reader.ToString(resolve(d, dict.Get("DA"))); ok {
		f.defaultAppearance = string(s)
	}
	if res, ok := d.GetDict(dict, "DR"); ok {
		f.resources = res
	}
	if q, ok := reader.ToInt(resolve(d, dict.Get("Q"))); ok {
		f.quadding = int(q)
	}
	f.readXFA(d, dict)
	roots, ok := reader.ToArray(resolve(d, dict.Get("Fields")))
	if !ok || len(roots) == 0 {
		// An AcroForm with no fields is usually a leftover: 561 of the 118 833
		// files in the figure corpus carry an empty one a producer forgot.
		//
		// Unless it carries an XFA package, and then it is the opposite — a
		// form whose fields live in the XML because that is where the whole
		// form lives. Those are exactly the documents a caller most needs told
		// about, and refusing them here is what made Dynamic answer for none
		// of the fourteen dynamic forms in a corpus of 2 240.
		if !f.hasXFA {
			return nil, false
		}
		return f, true
	}
	pages := f.pageNumbers()
	for _, entry := range roots {
		f.walk(entry, inherited{}, "", pages, 0)
	}
	if len(f.fields) == 0 {
		return nil, false
	}
	return f, true
}

// HasXFA says the document also carries Adobe's XML form description. Nothing
// here reads it: a file that has both is fillable through the standard half,
// which is the half this reads.
func (f *Form) HasXFA() bool { return f.hasXFA }

// Fields are every field that can hold a value, in the document's own order.
func (f *Form) Fields() []*Field { return f.fields }

// Field finds one by its whole name.
func (f *Form) Field(name string) (*Field, bool) {
	v, ok := f.byName[name]
	return v, ok
}

// inherited is what a field takes from its parents when it does not say a
// thing itself, which the format allows for nearly everything that matters.
type inherited struct {
	kind     reader.Name
	value    reader.Object
	flags    int64
	maxLen   int64
	quadding int64
	da       string
	options  reader.Object

	haveKind, haveValue, haveFlags bool
	haveMaxLen, haveQuadding       bool
	haveDA, haveOptions            bool
}

// walk goes down the field tree, gathering what is inherited on the way and
// making a Field of every node that can hold a value.
func (f *Form) walk(entry reader.Object, from inherited, prefix string, pages map[string]int, depth int) {
	if depth > maxFieldDepth {
		return
	}
	dict, ok := reader.ToDict(resolve(f.doc, entry))
	if !ok {
		return
	}
	here := from.with(f.doc, dict)
	name := prefix
	if part, ok := reader.ToString(resolve(f.doc, dict.Get("T"))); ok {
		if name != "" {
			name += "."
		}
		// A field's name is a text string like any other, and the forms that
		// matter write theirs in UTF-16: every field of an IRS form is called
		// something like topmostSubform[0].Page1[0].f1_01[0], written two
		// bytes to the character. Left as bytes it is unreadable and, worse,
		// unmatchable — nobody could name the field they wanted to fill.
		name += decodeText(part)
	}

	kids, hasKids := reader.ToArray(resolve(f.doc, dict.Get("Kids")))
	// A node whose kids have names of their own is a parent that only groups;
	// one whose kids are nameless is a field with several places on the page.
	grouping := false
	for _, k := range kids {
		kd, ok := reader.ToDict(resolve(f.doc, k))
		if !ok {
			continue
		}
		if _, named := kd["T"]; named {
			grouping = true
		}
	}
	if hasKids && grouping {
		for _, k := range kids {
			f.walk(k, here, name, pages, depth+1)
		}
		return
	}
	if !here.haveKind {
		// A node that is neither a field nor a parent of any is something else
		// the file put in the list.
		return
	}
	field := f.makeField(dict, entry, name, here)
	if hasKids {
		for _, k := range kids {
			field.addWidget(f, k, pages)
		}
	} else {
		// The widget is written into the field's own dictionary, which is what
		// a file does when a field shows in exactly one place — and nearly
		// every field does.
		field.addWidget(f, entry, pages)
	}
	f.fields = append(f.fields, field)
	if _, taken := f.byName[field.Name]; !taken {
		f.byName[field.Name] = field
	}
}

// with folds what a dictionary says into what its parents said.
func (i inherited) with(d *reader.Document, dict reader.Dict) inherited {
	out := i
	if v, ok := reader.ToName(resolve(d, dict.Get("FT"))); ok {
		out.kind, out.haveKind = v, true
	}
	if _, named := dict["V"]; named {
		out.value, out.haveValue = resolve(d, dict.Get("V")), true
	}
	if v, ok := reader.ToInt(resolve(d, dict.Get("Ff"))); ok {
		out.flags, out.haveFlags = v, true
	}
	if v, ok := reader.ToInt(resolve(d, dict.Get("MaxLen"))); ok {
		out.maxLen, out.haveMaxLen = v, true
	}
	if v, ok := reader.ToInt(resolve(d, dict.Get("Q"))); ok {
		out.quadding, out.haveQuadding = v, true
	}
	if v, ok := reader.ToString(resolve(d, dict.Get("DA"))); ok {
		out.da, out.haveDA = string(v), true
	}
	if _, named := dict["Opt"]; named {
		out.options, out.haveOptions = resolve(d, dict.Get("Opt")), true
	}
	return out
}

// The flag bits the format gives a field, counted from one as the tables do.
const (
	flagReadOnly    = 1 << 0
	flagRequired    = 1 << 1
	flagNoExport    = 1 << 2
	flagMultiline   = 1 << 12
	flagPassword    = 1 << 13
	flagNoToggleOff = 1 << 14
	flagRadio       = 1 << 15
	flagPushButton  = 1 << 16
	flagCombo       = 1 << 17
	flagEdit        = 1 << 18
	flagSort        = 1 << 19
	flagMultiSelect = 1 << 21
	flagComb        = 1 << 24
)

// makeField turns one node of the tree into a field.
func (f *Form) makeField(dict reader.Dict, entry reader.Object, name string, in inherited) *Field {
	flags := in.flags
	out := &Field{
		Name:              name,
		Kind:              kindOf(in.kind, flags),
		MaxLen:            int(in.maxLen),
		Quadding:          f.quadding,
		ReadOnly:          flags&flagReadOnly != 0,
		Required:          flags&flagRequired != 0,
		NoExport:          flags&flagNoExport != 0,
		Multiline:         flags&flagMultiline != 0,
		Password:          flags&flagPassword != 0,
		Comb:              flags&flagComb != 0,
		Sorted:            flags&flagSort != 0,
		MultiSelect:       flags&flagMultiSelect != 0,
		Editable:          flags&flagEdit != 0,
		defaultAppearance: f.defaultAppearance,
		form:              f,
		dict:              dict,
	}
	if in.haveQuadding {
		out.Quadding = int(in.quadding)
	}
	if in.haveDA {
		out.defaultAppearance = in.da
	}
	if ref, ok := entry.(reader.Ref); ok {
		out.ref = ref
	}
	if in.haveOptions {
		out.Options = f.readOptions(in.options)
	}
	if in.haveValue {
		out.Value, out.Values = f.readValue(in.value)
	}
	return out
}

// kindOf reads the sort of a field out of its type and its flags, since the
// format tells a radio button from a checkbox and a list from a drop-down by
// the flags alone.
func kindOf(t reader.Name, flags int64) Kind {
	switch t {
	case "Btn":
		switch {
		case flags&flagPushButton != 0:
			return PushButton
		case flags&flagRadio != 0:
			return Radio
		}
		return Checkbox
	case "Ch":
		if flags&flagCombo != 0 {
			return ComboBox
		}
		return ListBox
	case "Sig":
		return Signature
	}
	return Text
}

// readValue reads what a field holds, which may be a name, a string, or a
// list of either when several rows of a list box are chosen.
func (f *Form) readValue(v reader.Object) (string, []string) {
	if arr, ok := reader.ToArray(v); ok {
		var out []string
		for _, e := range arr {
			if s, ok := oneValue(f.doc, e); ok {
				out = append(out, s)
			}
		}
		if len(out) == 0 {
			return "", nil
		}
		return out[0], out
	}
	s, _ := oneValue(f.doc, v)
	return s, nil
}

// oneValue reads a single value, whichever of the two ways it is written.
func oneValue(d *reader.Document, o reader.Object) (string, bool) {
	v := resolve(d, o)
	if s, ok := reader.ToString(v); ok {
		return decodeText(s), true
	}
	if n, ok := reader.ToName(v); ok {
		return string(n), true
	}
	return "", false
}

// readOptions reads the rows of a choice field. A row is either the one string
// that is both what is stored and what is shown, or a pair that says them
// separately.
func (f *Form) readOptions(o reader.Object) []Option {
	arr, ok := reader.ToArray(o)
	if !ok {
		return nil
	}
	var out []Option
	for _, e := range arr {
		entry := resolve(f.doc, e)
		if pair, ok := reader.ToArray(entry); ok && len(pair) >= 2 {
			value, _ := oneValue(f.doc, pair[0])
			label, _ := oneValue(f.doc, pair[1])
			out = append(out, Option{Value: value, Label: label})
			continue
		}
		if s, ok := oneValue(f.doc, entry); ok {
			out = append(out, Option{Value: s, Label: s})
		}
	}
	return out
}

// addWidget records one place on a page where the field shows.
func (fld *Field) addWidget(f *Form, entry reader.Object, pages map[string]int) {
	dict, ok := reader.ToDict(resolve(f.doc, entry))
	if !ok {
		return
	}
	w := Widget{dict: dict}
	if ref, ok := entry.(reader.Ref); ok {
		w.ref = ref
		w.Page = pages[ref.String()]
	}
	if r, ok := rectangle(f.doc, dict.Get("Rect")); ok {
		w.Rect = r
	}
	if flags, ok := reader.ToInt(resolve(f.doc, dict.Get("F"))); ok {
		// The second bit is the one that says a thing is not to be shown, and
		// the sixth says it is not to be shown on a screen.
		w.Hidden = flags&(1<<1) != 0 || flags&(1<<5) != 0
	}
	w.On = onState(f.doc, dict)
	fld.Widgets = append(fld.Widgets, w)
}

// onState is the name a checkbox or radio widget uses for "chosen". It is
// whichever entry of the widget's normal appearance is not Off, and a set of
// radio buttons gives each of its widgets a different one, which is how the
// field's single value says which button was pressed.
func onState(d *reader.Document, dict reader.Dict) string {
	ap, ok := d.GetDict(dict, "AP")
	if !ok {
		return ""
	}
	normal, ok := d.GetDict(ap, "N")
	if !ok {
		return ""
	}
	best := ""
	for name := range normal {
		if name == "Off" {
			continue
		}
		// A dictionary has no order, so the name is chosen so that the same
		// file always gives the same answer.
		if best == "" || string(name) < best {
			best = string(name)
		}
	}
	return best
}

// pageNumbers says which page each annotation is on, by walking the pages once
// rather than asking each widget, since a widget's own /P is often missing.
func (f *Form) pageNumbers() map[string]int {
	out := map[string]int{}
	for i := 1; i <= f.doc.PageCount(); i++ {
		// A page within the count is one the reader has already walked to, so
		// there is nothing here that can fail; a dictionary that somehow came
		// back empty simply has no annotations in it.
		page, _ := f.doc.Page(i)
		annots, ok := reader.ToArray(resolve(f.doc, page.Get("Annots")))
		if !ok {
			continue
		}
		for _, a := range annots {
			if ref, ok := a.(reader.Ref); ok {
				out[ref.String()] = i
			}
		}
	}
	return out
}

// rectangle reads the four numbers a widget's place is written as, with the
// corners put the right way round: files do write them the other way.
func rectangle(d *reader.Document, o reader.Object) ([4]float64, bool) {
	var out [4]float64
	arr, ok := reader.ToArray(resolve(d, o))
	if !ok || len(arr) < 4 {
		return out, false
	}
	for i := 0; i < 4; i++ {
		v, ok := reader.ToFloat(resolve(d, arr[i]))
		if !ok {
			return out, false
		}
		out[i] = v
	}
	if out[0] > out[2] {
		out[0], out[2] = out[2], out[0]
	}
	if out[1] > out[3] {
		out[1], out[3] = out[3], out[1]
	}
	return out, true
}

// decodeText turns a PDF text string into one Go can hold. A form's values are
// text somebody typed, so they may be written either as bytes in the
// document's own eight-bit alphabet or as UTF-16 with a mark at the front.
func decodeText(b []byte) string {
	if len(b) >= 2 && b[0] == 0xFE && b[1] == 0xFF {
		var sb strings.Builder
		for i := 2; i+1 < len(b); i += 2 {
			r := rune(b[i])<<8 | rune(b[i+1])
			if r >= 0xD800 && r < 0xDC00 && i+3 < len(b) {
				low := rune(b[i+2])<<8 | rune(b[i+3])
				if low >= 0xDC00 && low < 0xE000 {
					sb.WriteRune(0x10000 + (r-0xD800)<<10 + (low - 0xDC00))
					i += 2
					continue
				}
			}
			sb.WriteRune(r)
		}
		return sb.String()
	}
	// Otherwise it is PDFDocEncoding, which agrees with Latin-1 everywhere a
	// form's values are likely to go.
	var sb strings.Builder
	for _, c := range b {
		sb.WriteRune(rune(c))
	}
	return sb.String()
}

// resolve follows an indirect reference, since a form is written almost
// entirely in them.
func resolve(d *reader.Document, o reader.Object) reader.Object {
	out, _ := d.Resolve(o)
	return out
}
