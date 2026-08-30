// Copyright (c) 2026, the go-pdfkit/forms authors
// All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package forms

import "github.com/go-pdfkit/reader"

// Packet is one part of a document's XFA package: a name and the XML under it.
//
// A package is split into parts — template, datasets, config, localeSet — and
// the two worth reading are template, which describes the form, and datasets,
// which holds what has been filled in.
type Packet struct {
	Name string
	Data []byte
}

// Dynamic says the document's pages are a placeholder and its real form exists
// only as XML.
//
// This is the difference that matters about XFA, and it is not the same
// question as [Form.HasXFA].
//
// A STATIC XFA form carries a full standard form beside the XML: the pages are
// drawn, the widgets are there, and everything in this package works on it. Of
// 1 499 forms in a corpus of 2 240 real documents, 560 carry XFA and 546 are
// of this kind — the XML is a second description of a form that is already
// readable, and reading the harder one gains nothing.
//
// A DYNAMIC one is different in kind. Its pages hold a panel reading "Please
// wait... your PDF viewer may not be able to display this type of document",
// and the form is laid out from the XML when it is opened. Fourteen of those
// 2 240 are like this. Every viewer but Adobe's own shows the panel: the
// format was removed from PDF 2.0, and neither poppler nor any browser lays it
// out.
//
// So a caller meeting one of these has a document that looks blank and is not.
// Saying so is worth more than drawing nothing quietly, which is what every
// tool including this one did before.
func (f *Form) Dynamic() bool { return f.dynamic }

// Packets are the parts of the XFA package, in the order the document names
// them, or nil when there is no package.
//
// The form cannot be laid out from here — that wants a layout engine this
// package does not have — but the values can be read: what has been filled in
// lives in the datasets part, as ordinary XML.
func (f *Form) Packets() []Packet { return f.packets }

// readXFA reads the package and whether the document says its pages are only a
// placeholder for it.
func (f *Form) readXFA(d *reader.Document, dict reader.Dict) {
	x := resolve(d, dict.Get("XFA"))
	switch v := x.(type) {
	case *reader.Stream:
		f.hasXFA = true
		if data, filter, err := reader.DecodeStream(v, d.Get); err == nil && filter == "" {
			f.packets = []Packet{{Name: "", Data: data}}
		}
	case reader.Array:
		f.hasXFA = true
		// The array runs name, stream, name, stream. A malformed one is read
		// as far as it makes sense rather than refused: a package missing its
		// config part still has its template.
		for i := 0; i+1 < len(v); i += 2 {
			name, _ := reader.ToString(resolve(d, v[i]))
			st, ok := reader.ToStream(resolve(d, v[i+1]))
			if !ok {
				continue
			}
			data, filter, err := reader.DecodeStream(st, d.Get)
			if err != nil || filter != "" {
				// Bytes still in a filter nothing here unpacks are not XML,
				// and handing them over as XML would be handing over noise.
				continue
			}
			f.packets = append(f.packets, Packet{Name: string(name), Data: data})
		}
	default:
		return
	}
	// /NeedsRendering on the catalogue is the document saying its pages are
	// not the form. It is the only thing in the file that distinguishes the
	// two kinds.
	if cat, err := d.Catalog(); err == nil {
		if b, ok := reader.ToBool(resolve(d, cat.Get("NeedsRendering"))); ok && bool(b) {
			f.dynamic = true
		}
	}
}
