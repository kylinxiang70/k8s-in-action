package main

import (
	"fmt"
)

func main() {
	result := make(map[string]interface{})
	result["description"] = "kylinxiang"
	if description, ok := result["description"].(string); ok {
		fmt.Println(description)
	}
}