package main

import (
	"encoding/json"
	"fmt"
)

// Student stores student info
type Student struct {
	ID   int
	Name string
}

func main() {
	s := `{"id": 1, "name": "kylinxiang"}`
	var student Student
	err := json.Unmarshal([]byte(s), &student)
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Printf("%v", student)
}
