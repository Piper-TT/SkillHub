package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

func main() {
	base := "http://localhost:18089"
	if len(os.Args) > 1 {
		base = os.Args[1]
	}

	prompt, err := os.ReadFile("cmd/fix_prompt/prompt.txt")
	if err != nil {
		fmt.Println("ERROR reading prompt.txt:", err)
		os.Exit(1)
	}

	desc := "查询CVE漏洞信息和补丁详情，基于漏洞数据库进行智能分析，支持自然语言查询"

	body := map[string]interface{}{
		"system_prompt": string(prompt),
		"description":   desc,
	}
	data, _ := json.Marshal(body)
	fmt.Printf("JSON length: %d, first 80 bytes: %s\n", len(data), string(data[:80]))

	req, _ := http.NewRequest("PUT", base+"/api/agent/2", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("ERROR:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	fmt.Println("success:", result["success"])
}
