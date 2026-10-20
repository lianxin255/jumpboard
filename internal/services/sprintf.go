package services

import "fmt"

// sprintf keeps the logger's files short.
func sprintf(format string, args ...any) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}
