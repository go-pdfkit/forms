// Package forms is the fillable half of a PDF: the boxes someone is meant to
// type into, the squares they tick, and the lists they choose from.
//
// A form is not drawn where it is defined. The document holds one list of
// fields, each field holds its value, and each field points at one or more
// widgets — the rectangles on the pages where it shows. What a filled-in field
// looks like is a little content stream of its own, called an appearance, and
// the awkward part of forms is that a field's appearance is not derived from
// its value when the page is drawn: it is written down beside the value, and
// whoever changes the value has to write a new one. A reader that fills a
// field and does not do that has filled in nothing anyone can see.
//
// So this package does three things. It reads the field tree — which is a tree,
// because a field may be a parent of others and inherits from wherever up the
// tree a thing was last said. It sets values. And it draws the appearance a
// value needs, from the default appearance string the document supplies, which
// is a fragment of content stream naming a font, a size and a colour, and in
// which a size of zero means "as large as fits".
//
// What it does not do is XFA, Adobe's XML form language: 66 of the 68 forms in
// the corpus this was measured against carry an XFA copy alongside the standard
// one, and every one of them is fillable through the standard one. XFA is a
// second, proprietary description of the same form, and a file that has both
// is not made more readable by reading the harder one.
package forms
