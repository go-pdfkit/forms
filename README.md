# forms

Read, fill, draw and **build** a PDF's **AcroForm** — the boxes somebody is
meant to type into, the squares they tick, the lists they choose from.

Pure Go, `CGO_ENABLED=0`, standard library plus the rest of
[go-pdfkit](https://github.com/go-pdfkit). Builds for `js/wasm` and for every
architecture the fleet targets.

```go
d, _ := reader.Open(b)
f, ok := forms.Read(d)
if !ok {
    // The document has no form. An AcroForm dictionary with an empty field
    // list is not a form: 561 of the 118 833 files in the figure corpus are
    // exactly that, left behind by a producer.
}
for _, field := range f.Fields() {
    fmt.Println(field.Name, field.Kind, field.Value)
}
f.Fill("f1_02", "Wolfgang Amadeus Mozart")
f.Fill("c1_1", "yes")
```

## Writing a form

The other direction: `FieldType`, `Flags`, `Dict`, `Check` and `CheckAll` say
what a field has to be in the file for the reader above to read back the field
you asked for. Whoever is writing the document adds the two things only a
writer can know — the widget references under `Kids`, and the page each widget
sits on.

```go
field := forms.Field{
    Name: "size", Kind: forms.ComboBox,
    Options: []forms.Option{{Value: "s", Label: "Small"}, {Value: "l", Label: "Large"}},
    Widgets: []forms.Widget{{Page: 1, Rect: [4]float64{72, 700, 300, 720}}},
}
if err := forms.CheckAll([]forms.Field{field}); err != nil {
    // A field with no name, a choice with no rows, a comb with no MaxLen, an
    // empty rectangle: each one makes a file that OPENS and is broken.
}
dict := forms.Dict(field) // FT, T, Ff, Q, MaxLen, Opt, V
```

It shares the reader's flag constants rather than restating them. The bits are
the one thing a second copy gets subtly wrong — bit 22 and bit 25 are the two
the tables make easy to miscount — and a writer that disagreed with the reader
would produce a form this package itself reads as something else, which no test
of either half alone could see. The round-trip tests write every kind and read
it back through `Read`.

A flag that does not belong to the kind is **dropped, not written**: `Multiline`
is bit 13 of a text field and `NoToggleToOff` is bit 15 of a button, so passing
one struct around with `Multiline` left set would otherwise make a checkbox that
cannot be unticked.

**Text is encoded as the reader decodes it.** `decodeText` reads a string with
no mark at the front as PDFDocEncoding — one rune per *byte* — so writing Go's
UTF-8 bytes turns `café` into `cafÃ©` in a file that opens and looks perfectly
well formed. Non-ASCII text is written as UTF-16 behind the byte-order mark;
text that is not valid UTF-8 is refused, because it cannot be stored and read
back under the name it was given. Found by fuzzing the round trip, not by
reading the code.

## What is awkward about forms

A field's value is not what gets drawn. What gets drawn is a little content
stream written down beside it, called an **appearance**, and whoever changes the
value has to write a new one. A reader that fills a field and does not do that
has filled in nothing anybody can see.

So this draws them. `Field.Appearance` gives back the stream a value needs,
built from the **default appearance string** the document supplies — a fragment
of content stream naming a font, a size and a colour — in which **a size of zero
means "as large as fits"**, which is what every form in the corpus asks for.

It handles one line and several, ranged left, centred and ranged right, **comb**
fields divided into one cell a character, list boxes with the chosen rows marked,
and the largest size that fits both the width and the height of the box.

A checkbox needs none of this: its widget already carries a drawing for each of
its states, so choosing among them is a matter of saying which. That is why
every one of the corpus's 2 015 checkboxes has an appearance dictionary and its
11 803 text fields mostly have none — an empty box has nothing to show.

## What it was measured against

The fleet's figure corpus is the wrong corpus for forms: of its 118 863 files,
561 carry an AcroForm and **every one of those has an empty field list**, and
there are 40 widget annotations in the whole of it. Nobody puts a fillable form
in a figure.

So a second one was fetched: **69 public forms, 68 with a form in them, 13 854
fields** — 11 803 text (232 of them several lines, 121 comb), 2 015 checkboxes,
34 push buttons, 2 signature fields; 9 968 quadded, 1 787 with a stated length,
1 435 read-only. Every field that would take one was filled and **10 391
appearances drawn**, every one naming a font the document actually carries.

That is what found a file whose fields name `HelveticaLTStd-Bold` 106 times and
which does not carry it. A stream naming a font nothing can find draws nothing
at all, so the form's own fallback is used instead.

## What it does not do

**XFA** — Adobe's XML form language. 66 of those 68 forms carry an XFA copy
beside the standard one, and every one of them is fillable through the standard
one. A file that has both is not made more readable by reading the harder one.
`Form.HasXFA` says when one is there.

Only one weight of each metric-compatible stand-in is carried, so a **bold**
standard face is measured as its regular where the document gives no widths of
its own. A bold string is a little wider than that, so an auto-sized bold value
comes out a little larger than Acrobat would make it — a point on a line of
text, stated here rather than hidden.

## Licence

BSD-3-Clause.
