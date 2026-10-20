package nginx

import "strconv"

// itoa is strconv.Itoa, kept local so the platform files need no import.
func itoa(n int) string { return strconv.Itoa(n) }
