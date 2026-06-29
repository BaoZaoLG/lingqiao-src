package cardcode

import "strings"

func Normalize(code string) string {
	code = strings.ToUpper(code)
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, " ", "")
	return code
}

func Same(a, b string) bool {
	return Normalize(a) == Normalize(b)
}
