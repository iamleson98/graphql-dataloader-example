package main

import (
	"encoding/json"
	"fmt"
	"log"
)

type User struct {
	Name string `json:"name"`
	Age  uint8  `json:"age"`

	IsDead bool
}

func main() {
	p := User{"Phuc", 20, true}

	data, err := json.MarshalIndent(p, "", "   ")
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(string(data))
}
