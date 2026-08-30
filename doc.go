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
// What it does not do is lay out XFA, Adobe's XML form language — and the
// reason has two halves, because the forms do.
//
// Measured over 2 240 real government forms: 1 499 carry a form, 560 of those
// carry an XFA package, and 546 of THOSE are static. A static one is a second,
// proprietary description of a form that is already there: the pages are
// drawn, the widgets exist, and everything here works on it. Such a file is
// not made more readable by reading the harder description.
//
// The other fourteen are dynamic. Their pages hold a panel reading "Please
// wait... your PDF viewer may not be able to display this type of document",
// and the form is laid out from the XML when it is opened — by Adobe's reader
// and by nothing else, the format having been removed from PDF 2.0. Laying one
// out means an XFA layout engine; the reference implementation, pdf.js, spends
// 395 kilobytes of JavaScript on it.
//
// So this says which kind a document is, through [Form.Dynamic], and hands
// back the XML through [Form.Packets] — the values a form has been filled with
// live in its datasets part, as ordinary XML. What it will not do is draw
// nothing and say nothing, which is what every tool including this one did
// before: a document that looks blank and is not.
package forms
