package test

import "fmt"

type MyStruct struct {
	Name string
}

func helper() {
	fmt.Println("helper")
}

func MainFunc() {
	s := MyStruct{Name: "test"}
	_ = s
	helper()
}
