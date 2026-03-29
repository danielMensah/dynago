package dynago

import "strings"

// SplitKey splits a composite key string by the given delimiter.
// Teams use different delimiters (#, |, ::, etc.) so the delimiter is a parameter.
func SplitKey(key string, delimiter string) []string {
	return strings.Split(key, delimiter)
}
