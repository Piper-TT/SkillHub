package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	path := "internal/middleware/security.go"
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("Read error:", err)
		return
	}

	content := string(data)
	old := `if !strings.HasSuffix(c.Request.URL.Path, "/upload") {`
	new := `if !strings.HasSuffix(c.Request.URL.Path, "/upload") || strings.HasPrefix(c.Request.URL.Path, "/api/server/") {`

	if !strings.Contains(content, old) {
		fmt.Println("Pattern not found")
		return
	}
	if strings.Contains(content, new) {
		fmt.Println("Already patched")
		return
	}

	content = strings.Replace(content, old, new, 1)
	err = os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		fmt.Println("Write error:", err)
		return
	}
	fmt.Println("Patched successfully")
}
