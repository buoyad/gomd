package main

import (
	"fmt"
	"io/ioutil"

	"./parser"
)

func main() {
	f, _ := ioutil.ReadFile("test.md")
	m := parser.Lex("test", string(f))
	fmt.Println(m)
}
