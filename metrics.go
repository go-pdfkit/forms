package forms

import (
	"strings"
	"sync"

	"github.com/go-opentype/fonts/arimo"
	"github.com/go-opentype/fonts/cousine"
	"github.com/go-opentype/fonts/tinos"
	"github.com/go-opentype/opentype"
	"github.com/go-pdfkit/pdffont"
	"github.com/go-pdfkit/reader"
)

// Drawing a value into a box means knowing how wide the value is: to centre
// it, to range it right, and above all to work out what size it has to be,
// since a form's default appearance nearly always asks for a size of zero,
// which means "as large as fits".
//
// The document's own font is asked first, because it is the one that will
// actually be used and it carries its own widths. Where it gives none — the
// standard fourteen faces are described by name and nothing else — one of the
// metric-compatible stand-ins is measured instead: Arimo, Tinos and Cousine
// have the same advances as Helvetica, Times and Courier, glyph for glyph.
//
// Only one weight of each stand-in is carried, so a bold standard face is
// measured as its regular. A bold string is a little wider than that, so an
// auto-sized bold value comes out a little larger than Acrobat would make it.
// That is a difference of a point on a line of text, and it is stated here
// rather than hidden.
type metrics struct {
	font *pdffont.Font
	// code says which byte of the font's encoding shows each character, built
	// by asking the font what each of its 256 codes stands for and turning
	// the answer round.
	code map[rune]byte
	// stand is the face measured where the document gives no width.
	stand *opentype.Font
	perEm float64
}

// A stand-in, read once and shared.
type standIn struct {
	once sync.Once
	ttf  []byte
	font *opentype.Font
}

func (s *standIn) get() *opentype.Font {
	s.once.Do(func() { s.font, _ = opentype.Parse(s.ttf) })
	return s.font
}

var (
	sansStandIn  = &standIn{ttf: arimo.TTF}
	serifStandIn = &standIn{ttf: tinos.TTF}
	monoStandIn  = &standIn{ttf: cousine.TTF}
)

// measurerFor builds one from a face alone, for the cases where there is no
// document font to ask.
func (f *Form) measurerFor(s *standIn) *metrics {
	m := &metrics{code: map[rune]byte{}, stand: s.get()}
	if m.stand != nil {
		m.perEm = float64(m.stand.UnitsPerEm())
	}
	return m
}

// measurer builds something that can measure the text a field will show, from
// the font its default appearance names.
func (f *Form) measurer(name string) *metrics {
	m := &metrics{code: map[rune]byte{}}
	if dict, ok := f.fontDict(name); ok {
		m.font = pdffont.Read(f.doc, dict)
	}
	if m.font != nil {
		for c := 0; c < 256; c++ {
			s, ok := m.font.Text(c)
			if !ok {
				continue
			}
			r := []rune(s)
			if len(r) != 1 {
				continue
			}
			if _, taken := m.code[r[0]]; !taken {
				m.code[r[0]] = byte(c)
			}
		}
	}
	m.stand = standInFor(name).get()
	if m.stand != nil {
		m.perEm = float64(m.stand.UnitsPerEm())
	}
	return m
}

// fontDict finds the font a default appearance names, in the form's own
// resource dictionary.
func (f *Form) fontDict(name string) (reader.Dict, bool) {
	if f.resources == nil || name == "" {
		return nil, false
	}
	fonts, ok := f.doc.GetDict(f.resources, "Font")
	if !ok {
		return nil, false
	}
	return f.doc.GetDict(fonts, reader.Name(name))
}

// standInFor picks a face by the name a default appearance uses. The short
// names are the ones a form's resources give the standard fourteen.
func standInFor(name string) *standIn {
	l := strings.ToLower(name)
	switch {
	case strings.HasPrefix(l, "cour"), strings.HasPrefix(l, "co"), strings.Contains(l, "mono"):
		return monoStandIn
	case strings.HasPrefix(l, "ti"), strings.Contains(l, "times"), strings.Contains(l, "roman"),
		strings.Contains(l, "serif") && !strings.Contains(l, "sans"):
		return serifStandIn
	}
	return sansStandIn
}

// encode turns a value into the bytes the field's font shows it as. A
// character the font has no code for is left out rather than shown as
// something else.
func (m *metrics) encode(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if c, ok := m.code[r]; ok {
			out = append(out, c)
			continue
		}
		// A font that said nothing about its encoding is taken to be the
		// eight-bit one every such font is, which is what the format's own
		// default says.
		if len(m.code) == 0 && r < 256 {
			out = append(out, byte(r))
		}
	}
	return out
}

// width is how wide a string is when set at one point, in points.
func (m *metrics) width(s string) float64 {
	total := 0.0
	for _, r := range s {
		total += m.runeWidth(r)
	}
	return total
}

// runeWidth is one character's advance, from the document's own widths where
// it has them and from the stand-in where it has not.
func (m *metrics) runeWidth(r rune) float64 {
	if m.font != nil {
		if c, ok := m.code[r]; ok && m.font.HasWidth(int(c)) {
			return m.font.Width(int(c))
		}
	}
	if m.stand == nil || m.perEm == 0 {
		// Nothing to measure with: half an em a character is what a
		// typewriter would do, and it keeps a value inside its box.
		return 0.5
	}
	gid, ok := m.stand.GlyphIndex(r)
	if !ok {
		return 0.5
	}
	return float64(m.stand.GlyphAdvance(gid)) / m.perEm
}

// height is how far apart the top and the bottom of the face are, which is
// what decides the largest size that fits in a box.
func (m *metrics) height() (ascent, descent float64) {
	if m.stand == nil || m.perEm == 0 {
		return 0.75, 0.25
	}
	// The three faces are ours and embedded, so what they say about themselves
	// is known to be sensible; a face that would not parse has been dealt with
	// above, where there is nothing to ask.
	return float64(m.stand.Ascent()) / m.perEm, -float64(m.stand.Descent()) / m.perEm
}
