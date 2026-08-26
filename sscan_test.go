package forms

import "fmt"

// fmtSscan is fmt.Sscan under a name that says what the tests use it for.
func fmtSscan(s string, v *float64) (int, error) { return fmt.Sscan(s, v) }
