package forms

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-pdfkit/reader"
)

// An Appearance is the little content stream that shows a field's value, and
// everything needed to put it in the document.
type Appearance struct {
	// Content is the stream itself.
	Content []byte
	// BBox is the box it is drawn in, which is the widget's own size with its
	// corner at the origin: an appearance is drawn in a space of its own and
	// placed by the widget's rectangle.
	BBox [4]float64
	// FontName is the name Content uses for its font, and Font the object in
	// the form's resource dictionary that name stands for. Whoever writes the
	// stream out has to give it a resource dictionary saying so.
	FontName string
	Font     reader.Object
}

// defaultDA is what a form with nothing to say about appearance gets: black
// text in a sans face, at whatever size fits.
const defaultDA = "/Helv 0 Tf 0 g"

// Appearance draws the stream that shows what a field holds, for one of its
// widgets.
//
// A checkbox or a radio button does not need one: its widget already carries a
// drawing for every state it has, and choosing among them is a matter of
// saying which — which is why every one of the 2 015 checkboxes in the corpus
// has an appearance dictionary and 11 803 text fields mostly have none, since
// an empty box has nothing to show.
func (f *Field) Appearance(w Widget) (Appearance, bool) {
	switch f.Kind {
	case Text, ComboBox:
	case ListBox:
	default:
		return Appearance{}, false
	}
	width := w.Rect[2] - w.Rect[0]
	height := w.Rect[3] - w.Rect[1]
	if width <= 0 || height <= 0 {
		return Appearance{}, false
	}
	da := f.defaultAppearance
	if strings.TrimSpace(da) == "" {
		da = defaultDA
	}
	name, size, rest := splitDA(da)
	// A field may name a font the document does not supply — 106 of the
	// corpus's fields name HelveticaLTStd-Bold and one of its files does not
	// carry it. A stream that names a font nothing can find draws nothing at
	// all, so the form's own fallback is used instead, and failing that the
	// writer is left to supply a standard face under the name.
	font, haveFont := f.form.fontDict(name)
	if !haveFont {
		if alt, ok := f.form.fontDict("Helv"); ok {
			name, font, haveFont = "Helv", alt, true
		}
	}
	m := f.form.measurer(name)
	pad := f.padding(w)

	var body []byte
	switch {
	case f.Kind == ListBox:
		body = f.drawList(m, name, size, rest, width, height, pad)
	case f.Multiline:
		body = f.drawWrapped(m, name, size, rest, width, height, pad)
	case f.Comb && f.MaxLen > 0:
		body = f.drawComb(m, name, size, rest, width, height, pad)
	default:
		body = f.drawLine(m, name, size, rest, width, height, pad)
	}

	var out bytes.Buffer
	// The marked content is what says this drawing belongs to a form field,
	// which is what lets a later reader tell it from the page under it.
	out.WriteString("/Tx BMC\nq\n")
	fmt.Fprintf(&out, "%s %s %s %s re W n\n",
		number(pad/2), number(pad/2), number(width-pad), number(height-pad))
	out.Write(body)
	out.WriteString("Q\nEMC\n")

	app := Appearance{Content: out.Bytes(), BBox: [4]float64{0, 0, width, height}, FontName: name}
	if haveFont {
		app.Font = font
	}
	return app, true
}

// padding is how far in from the edge the text starts: the border the widget
// draws, and one point besides, which is what every reader leaves.
func (f *Field) padding(w Widget) float64 {
	border := 1.0
	if bs, ok := f.form.doc.GetDict(w.dict, "BS"); ok {
		if v, ok := reader.ToFloat(resolve(f.form.doc, bs.Get("W"))); ok {
			border = v
		}
	}
	return 2 * (border + 1)
}

// drawLine draws one line of text, at whatever size was asked for or the
// largest that fits, ranged as the field asks.
func (f *Field) drawLine(m *metrics, font string, size float64, rest string, width, height, pad float64) []byte {
	text := f.Text()
	inner := width - pad
	size = f.sizeFor(m, size, text, inner, height-pad)
	w := m.width(text) * size
	x := pad / 2
	switch f.Quadding {
	case 1:
		x = (width - w) / 2
	case 2:
		x = width - pad/2 - w
	}
	if x < pad/2 {
		x = pad / 2
	}
	ascent, descent := m.height()
	// The line sits so that what is above the baseline and what is below it
	// are equally far from the top and the bottom of the box.
	y := (height-(ascent+descent)*size)/2 + descent*size
	return f.show(m, font, size, rest, text, x, y)
}

// drawComb draws a field divided into equal cells, one character to each,
// which is how a form asks for a code a digit at a time.
func (f *Field) drawComb(m *metrics, font string, size float64, rest string, width, height, pad float64) []byte {
	text := f.Text()
	cell := width / float64(f.MaxLen)
	size = f.sizeFor(m, size, "0", cell, height-pad)
	ascent, descent := m.height()
	y := (height-(ascent+descent)*size)/2 + descent*size
	var out bytes.Buffer
	for i, r := range []rune(text) {
		if i >= f.MaxLen {
			break
		}
		s := string(r)
		x := float64(i)*cell + (cell-m.width(s)*size)/2
		out.Write(f.show(m, font, size, rest, s, x, y))
	}
	return out.Bytes()
}

// drawWrapped draws a field that takes more than one line, breaking the value
// where it will not fit and honouring the newlines somebody typed.
func (f *Field) drawWrapped(m *metrics, font string, size float64, rest string, width, height, pad float64) []byte {
	inner := width - pad
	if size <= 0 {
		size = 12
		// A multi-line field asked to size itself is made small enough that
		// what it holds fits in it, down to a size nobody could read.
		for size > 4 && float64(len(f.wrap(m, size, inner)))*size*1.15 > height-pad {
			size -= 0.5
		}
	}
	lines := f.wrap(m, size, inner)
	ascent, _ := m.height()
	leading := size * 1.15
	y := height - pad/2 - ascent*size
	var out bytes.Buffer
	for _, line := range lines {
		if y < -leading {
			break
		}
		x := pad / 2
		switch f.Quadding {
		case 1:
			x = (width - m.width(line)*size) / 2
		case 2:
			x = width - pad/2 - m.width(line)*size
		}
		if x < pad/2 {
			x = pad / 2
		}
		out.Write(f.show(m, font, size, rest, line, x, y))
		y -= leading
	}
	return out.Bytes()
}

// drawList draws the rows of a list box, with the chosen ones behind a band of
// the colour a reader uses for a choice.
func (f *Field) drawList(m *metrics, font string, size float64, rest string, width, height, pad float64) []byte {
	if size <= 0 {
		size = 12
	}
	chosen := map[string]bool{}
	for _, v := range f.selected() {
		chosen[v] = true
	}
	ascent, _ := m.height()
	leading := size * 1.15
	y := height - pad/2 - ascent*size
	var out bytes.Buffer
	for _, o := range f.Options {
		if y < -leading {
			break
		}
		if chosen[o.Value] {
			// The band every reader draws behind a chosen row.
			fmt.Fprintf(&out, "q 0.6 0.756 0.854 rg %s %s %s %s re f Q\n",
				number(pad/2), number(y-leading+ascent*size*0.25),
				number(width-pad), number(leading))
		}
		out.Write(f.show(m, font, size, rest, o.Label, pad/2, y))
		y -= leading
	}
	return out.Bytes()
}

// selected is the rows a choice field holds, which may be one or several.
func (f *Field) selected() []string {
	if len(f.Values) > 0 {
		return f.Values
	}
	if f.Value == "" {
		return nil
	}
	return []string{f.Value}
}

// wrap breaks a value into the lines it will be shown on, at the spaces where
// it can be broken and at the newlines somebody typed.
func (f *Field) wrap(m *metrics, size, width float64) []string {
	var out []string
	for _, para := range strings.Split(strings.ReplaceAll(f.Text(), "\r\n", "\n"), "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		line := words[0]
		for _, word := range words[1:] {
			if m.width(line+" "+word)*size <= width {
				line += " " + word
				continue
			}
			out = append(out, line)
			line = word
		}
		out = append(out, line)
	}
	return out
}

// sizeFor is the size to set text at: the one asked for, or the largest that
// fits both ways when the default appearance asked for nothing, which is what
// a size of zero means and what nearly every form asks for.
func (f *Field) sizeFor(m *metrics, size float64, text string, width, height float64) float64 {
	if size > 0 {
		return size
	}
	ascent, descent := m.height()
	fit := height / (ascent + descent)
	if w := m.width(text); w > 0 && width > 0 {
		if byWidth := width / w; byWidth < fit {
			fit = byWidth
		}
	}
	// A field of no useful size still gets something drawable, and nothing is
	// set larger than a form would ever ask for.
	if fit < 1 {
		fit = 1
	}
	if fit > 144 {
		fit = 144
	}
	return fit
}

// show is one run of text put down at a place.
func (f *Field) show(m *metrics, font string, size float64, rest, text string, x, y float64) []byte {
	var out bytes.Buffer
	out.WriteString("BT\n")
	fmt.Fprintf(&out, "/%s %s Tf\n", font, number(size))
	if strings.TrimSpace(rest) != "" {
		out.WriteString(strings.TrimSpace(rest))
		out.WriteString("\n")
	}
	fmt.Fprintf(&out, "1 0 0 1 %s %s Tm\n", number(x), number(y))
	out.Write(literal(m.encode(text)))
	out.WriteString(" Tj\nET\n")
	return out.Bytes()
}

// splitDA reads a default appearance string, which is a scrap of content
// stream: a font and a size, and then whatever else it wants to say — a
// colour, nearly always. What is wanted is the font, the size, and the rest
// kept as it was, since a colour is written a dozen ways and copying it is
// safer than reading it.
func splitDA(da string) (name string, size float64, rest string) {
	fields := strings.Fields(da)
	keep := make([]string, 0, len(fields))
	for i := 0; i < len(fields); i++ {
		if fields[i] == "Tf" && i >= 2 {
			name = strings.TrimPrefix(fields[i-2], "/")
			size, _ = strconv.ParseFloat(fields[i-1], 64)
			// The two operands belong to the Tf and go with it.
			keep = keep[:max(0, len(keep)-2)]
			continue
		}
		keep = append(keep, fields[i])
	}
	if name == "" {
		name = "Helv"
	}
	return name, size, strings.Join(keep, " ")
}

// literal writes a string the way a content stream holds one, with the three
// characters that would otherwise end it escaped.
func literal(b []byte) []byte {
	var out bytes.Buffer
	out.WriteByte('(')
	for _, c := range b {
		switch c {
		case '(', ')', '\\':
			out.WriteByte('\\')
			out.WriteByte(c)
		case '\r':
			out.WriteString("\\r")
		case '\n':
			out.WriteString("\\n")
		default:
			out.WriteByte(c)
		}
	}
	out.WriteByte(')')
	return out.Bytes()
}

// number writes a number the way a content stream does: as short as it can be
// without losing anything that matters at the size of a page.
func number(v float64) string {
	s := strconv.FormatFloat(v, 'f', 4, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimSuffix(s, ".")
	if s == "" || s == "-" || s == "-0" {
		return "0"
	}
	return s
}
