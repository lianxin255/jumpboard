package services

import "strconv"

// itoa keeps the platform-neutral files free of the strconv import.
func itoa(n int) string { return strconv.Itoa(n) }
