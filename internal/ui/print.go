package ui

import (
	"fmt"
	"os"
)

func Success(msg string) {
	fmt.Fprintln(os.Stdout, SuccessStyle.Render("✓ "+msg))
}

func Successf(format string, args ...interface{}) {
	Success(fmt.Sprintf(format, args...))
}

func Error(msg string) {
	fmt.Fprintln(os.Stderr, ErrorStyle.Render("✗ "+msg))
}

func Errorf(format string, args ...interface{}) {
	Error(fmt.Sprintf(format, args...))
}

func Warning(msg string) {
	fmt.Fprintln(os.Stdout, WarningStyle.Render("⚠ "+msg))
}

func Warningf(format string, args ...interface{}) {
	Warning(fmt.Sprintf(format, args...))
}

func Info(msg string) {
	fmt.Fprintln(os.Stdout, InfoStyle.Render("ℹ "+msg))
}

func Infof(format string, args ...interface{}) {
	Info(fmt.Sprintf(format, args...))
}

func Print(msg string) {
	fmt.Fprintln(os.Stdout, msg)
}

func Printf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stdout, format, args...)
}

func Println(msg string) {
	fmt.Fprintln(os.Stdout, msg)
}

func Title(title string) {
	fmt.Fprintln(os.Stdout, TitleStyle.Render(title))
}

func Header(header string) {
	fmt.Fprintln(os.Stdout, HeaderStyle.Render(header))
}

func Subtitle(subtitle string) {
	fmt.Fprintln(os.Stdout, SubtitleStyle.Render(subtitle))
}

func Dim(text string) string {
	return DimStyle.Render(text)
}

func Bold(text string) string {
	return BoldStyle.Render(text)
}

func Highlight(text string) string {
	return HighlightStyle.Render(text)
}

func Muted(text string) string {
	return MutedStyle.Render(text)
}
