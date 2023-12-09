// Package main ...
package main

import (
	"fmt"

	"github.com/Fromsko/rodPro/lib/launcher"
	"github.com/Fromsko/rodPro/lib/utils"
)

func main() {
	p, err := launcher.NewBrowser().Get()
	utils.E(err)

	fmt.Println(p)
}
