package main

import (
	// "fmt"
	"io/ioutil"

	"./parser"
)

func main() {
	f, _ := ioutil.ReadFile("test.md")
	parser.Lex("test", string(f))
	// fmt.Println(m)
}
