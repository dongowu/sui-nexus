package gateway

func shortID(value string) string {
	if len(value) <= 16 {
		return value
	}
	return value[:16]
}
