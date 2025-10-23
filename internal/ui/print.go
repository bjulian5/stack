package ui

import (
	"fmt"
	"os"
)

// Success prints a success message with a checkmark icon
func Success(msg string) {
	fmt.Fprintln(os.Stdout, SuccessStyle.Render("✓ "+msg))
}

// Successf prints a formatted success message with a checkmark icon
func Successf(format string, args ...interface{}) {
	Success(fmt.Sprintf(format, args...))
}

// Error prints an error message with an X icon
func Error(msg string) {
	fmt.Fprintln(os.Stderr, ErrorStyle.Render("✗ "+msg))
}

// Errorf prints a formatted error message with an X icon
func Errorf(format string, args ...interface{}) {
	Error(fmt.Sprintf(format, args...))
}

// Warning prints a warning message with a warning icon
func Warning(msg string) {
	fmt.Fprintln(os.Stdout, WarningStyle.Render("⚠ "+msg))
}

// Warningf prints a formatted warning message with a warning icon
func Warningf(format string, args ...interface{}) {
	Warning(fmt.Sprintf(format, args...))
}

// Info prints an info message with an info icon
func Info(msg string) {
	fmt.Fprintln(os.Stdout, InfoStyle.Render("ℹ "+msg))
}

// Infof prints a formatted info message with an info icon
func Infof(format string, args ...interface{}) {
	Info(fmt.Sprintf(format, args...))
}

// Print prints a plain message (no styling)
func Print(msg string) {
	fmt.Fprintln(os.Stdout, msg)
}

// Printf prints a formatted plain message (no styling)
func Printf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stdout, format, args...)
}

// Println prints a plain message with a newline (no styling)
func Println(msg string) {
	fmt.Fprintln(os.Stdout, msg)
}

// Title prints a large title with background
func Title(title string) {
	fmt.Fprintln(os.Stdout, TitleStyle.Render(title))
}

// Header prints a header (bold, colored, no background)
func Header(header string) {
	fmt.Fprintln(os.Stdout, HeaderStyle.Render(header))
}

// Subtitle prints a subtitle (italic, muted)
func Subtitle(subtitle string) {
	fmt.Fprintln(os.Stdout, SubtitleStyle.Render(subtitle))
}

// Dim prints dimmed/muted text
func Dim(text string) string {
	return DimStyle.Render(text)
}

// Bold prints bold text
func Bold(text string) string {
	return BoldStyle.Render(text)
}

// Highlight prints highlighted text (primary color, bold)
func Highlight(text string) string {
	return HighlightStyle.Render(text)
}

// Muted prints muted text
func Muted(text string) string {
	return MutedStyle.Render(text)
}
