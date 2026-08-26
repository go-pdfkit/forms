package forms

import (
	"errors"
	"fmt"
	"strings"
)

// Setting a value is the easy half of filling in a form. The hard half is that
// the value is not what gets drawn: what gets drawn is a little content stream
// written down beside it, and a reader that sets one without the other has
// filled in nothing anybody can see. So every setter here marks the field as
// changed, and whoever writes the document out asks for the appearance.

// ErrReadOnly is what a field says when the document asks that it not be
// filled in.
var ErrReadOnly = errors.New("forms: the field is read-only")

// ErrKind is what a field says when it is asked for a value of the wrong sort
// — a checkbox given a sentence, or a text field ticked.
var ErrKind = errors.New("forms: the field does not hold that sort of value")

// SetText puts a string in a text field, or in a combo box that takes one.
func (f *Field) SetText(s string) error {
	if f.ReadOnly {
		return fmt.Errorf("%q: %w", f.Name, ErrReadOnly)
	}
	switch f.Kind {
	case Text:
	case ComboBox:
		if !f.Editable && !f.hasOption(s) {
			return fmt.Errorf("%q: %q is not one of its rows and it takes no other: %w", f.Name, s, ErrKind)
		}
	default:
		return fmt.Errorf("%q is a %s: %w", f.Name, f.Kind, ErrKind)
	}
	if f.MaxLen > 0 && len([]rune(s)) > f.MaxLen {
		s = string([]rune(s)[:f.MaxLen])
	}
	if !f.Multiline {
		// A single-line field holds one line, and a newline in it would run
		// off the end of the box rather than wrap.
		s = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(s)
	}
	f.Value, f.Values, f.changed = s, nil, true
	return nil
}

// SetChecked ticks or unticks a checkbox, and presses a radio button.
//
// A ticked box does not hold "true": it holds the name of the state its widget
// calls chosen, which every widget names for itself. That is how a row of
// radio buttons sharing one value says which of them was pressed.
func (f *Field) SetChecked(on bool) error {
	if f.ReadOnly {
		return fmt.Errorf("%q: %w", f.Name, ErrReadOnly)
	}
	if f.Kind != Checkbox && f.Kind != Radio {
		return fmt.Errorf("%q is a %s: %w", f.Name, f.Kind, ErrKind)
	}
	if !on {
		f.Value, f.Values, f.changed = "Off", nil, true
		return nil
	}
	state := ""
	for _, w := range f.Widgets {
		if w.On != "" {
			state = w.On
			break
		}
	}
	if state == "" {
		// A widget with no appearance dictionary has no name for chosen. Yes
		// is what the format uses when nothing else says otherwise.
		state = "Yes"
	}
	f.Value, f.Values, f.changed = state, nil, true
	return nil
}

// Press chooses one particular button of a radio group by the state its widget
// names, which is what to use when a group has more than two buttons.
func (f *Field) Press(state string) error {
	if f.ReadOnly {
		return fmt.Errorf("%q: %w", f.Name, ErrReadOnly)
	}
	if f.Kind != Checkbox && f.Kind != Radio {
		return fmt.Errorf("%q is a %s: %w", f.Name, f.Kind, ErrKind)
	}
	if state != "Off" && !f.hasState(state) {
		return fmt.Errorf("%q has no button called %q: %w", f.Name, state, ErrKind)
	}
	f.Value, f.Values, f.changed = state, nil, true
	return nil
}

// States are the names the field's buttons answer to, which is what Press
// takes. Off is not among them; it is what every button is when none is
// pressed.
func (f *Field) States() []string {
	var out []string
	seen := map[string]bool{}
	for _, w := range f.Widgets {
		if w.On != "" && !seen[w.On] {
			seen[w.On] = true
			out = append(out, w.On)
		}
	}
	return out
}

// Choose picks rows of a choice field. A list box that says it takes several
// takes several; anything else takes one.
func (f *Field) Choose(values ...string) error {
	if f.ReadOnly {
		return fmt.Errorf("%q: %w", f.Name, ErrReadOnly)
	}
	if f.Kind != ComboBox && f.Kind != ListBox {
		return fmt.Errorf("%q is a %s: %w", f.Name, f.Kind, ErrKind)
	}
	if len(values) > 1 && !f.MultiSelect {
		return fmt.Errorf("%q takes one row at a time: %w", f.Name, ErrKind)
	}
	for _, v := range values {
		if !f.hasOption(v) && !(f.Kind == ComboBox && f.Editable) {
			return fmt.Errorf("%q is not a row of %q: %w", v, f.Name, ErrKind)
		}
	}
	f.changed = true
	switch len(values) {
	case 0:
		f.Value, f.Values = "", nil
	case 1:
		f.Value, f.Values = values[0], nil
	default:
		f.Value, f.Values = values[0], append([]string(nil), values...)
	}
	return nil
}

// Checked says whether a box is ticked or a button pressed.
func (f *Field) Checked() bool { return f.Value != "" && f.Value != "Off" }

// Changed says a value was put here rather than read from the file, which is
// what tells whoever writes the document out what has to be written.
func (f *Field) Changed() bool { return f.changed }

// Text is what the field shows: its value, or for a choice field the label
// its value stands for, since a file may store one thing and show another.
func (f *Field) Text() string {
	if f.Kind == ComboBox || f.Kind == ListBox {
		for _, o := range f.Options {
			if o.Value == f.Value {
				return o.Label
			}
		}
	}
	return f.Value
}

// hasOption says whether a choice field has that row.
func (f *Field) hasOption(v string) bool {
	for _, o := range f.Options {
		if o.Value == v || o.Label == v {
			return true
		}
	}
	return false
}

// hasState says whether any of the field's widgets answers to that name.
func (f *Field) hasState(s string) bool {
	for _, w := range f.Widgets {
		if w.On == s {
			return true
		}
	}
	return false
}

// Changed lists the fields somebody has filled in, which is what a writer
// needs and nothing more.
func (f *Form) Changed() []*Field {
	var out []*Field
	for _, fld := range f.fields {
		if fld.changed {
			out = append(out, fld)
		}
	}
	return out
}

// Fill sets a field by name, choosing the right sort of setter for what it is,
// which is what a command line or a map of values wants.
func (f *Form) Fill(name, value string) error {
	fld, ok := f.byName[name]
	if !ok {
		return fmt.Errorf("forms: no field called %q", name)
	}
	switch fld.Kind {
	case Checkbox, Radio:
		switch strings.ToLower(value) {
		case "", "off", "no", "false", "0":
			return fld.SetChecked(false)
		case "on", "yes", "true", "1":
			return fld.SetChecked(true)
		}
		return fld.Press(value)
	case ComboBox, ListBox:
		return fld.Choose(value)
	case PushButton:
		return fmt.Errorf("%q is a button, not a thing to fill in: %w", name, ErrKind)
	case Signature:
		return fmt.Errorf("%q is a signature field: %w", name, ErrKind)
	}
	return fld.SetText(value)
}
