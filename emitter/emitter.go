package emitter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// EmitChanges sends a json payload of cluster changes to a remote endpoint
func EmitChanges(newData interface{}, url string) {
	jsonBody, err := json.Marshal(newData)
	if err != nil {
		log.Println("Error marshalling new data", err)
	}

	fmt.Printf("URL %v\n", url)

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error:", err)
	}
	defer resp.Body.Close()
}
