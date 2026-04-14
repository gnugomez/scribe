package cmd

const (
	colorGreen = "\033[32m"
	colorReset = "\033[0m"
)

func green(s string) string {
	return colorGreen + s + colorReset
}
