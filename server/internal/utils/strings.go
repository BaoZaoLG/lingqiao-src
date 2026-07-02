package utils

func Short(s string, l int) string {
	if len(s) <= l {
		return s
	}
	return s[:l]
}
