package parser

import "fmt"

func Lex(name, input string) []item {
	_, items := lex(name, input)
	res := make([]item, 20)
	i := 0
	for elem := range items {
		res[i] = elem
		fmt.Println(elem)
		i++
	}
	return res
}
