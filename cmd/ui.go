package cmd

import "github.com/fatih/color"

var (
	successStyle = color.New(color.FgGreen, color.Bold)
	errorStyle   = color.New(color.FgRed, color.Bold)
	warnStyle    = color.New(color.FgYellow, color.Bold)
	dimStyle     = color.New(color.Faint)
	boldStyle    = color.New(color.Bold)
)

func success(s string) string { return successStyle.Sprint(s) }
func errText(s string) string { return errorStyle.Sprint(s) }
func warn(s string) string    { return warnStyle.Sprint(s) }
func dim(s string) string     { return dimStyle.Sprint(s) }
func bold(s string) string    { return boldStyle.Sprint(s) }
