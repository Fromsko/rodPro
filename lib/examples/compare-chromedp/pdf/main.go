// Package main ...
package main

import (
	"fmt"

	"github.com/Fromsko/rodPro"
)

func main() {
	rod.New().MustConnect().MustPage("https://www.google.com/").MustWaitLoad().MustPDF("sample.pdf")
	fmt.Println("wrote sample.pdf")
}
