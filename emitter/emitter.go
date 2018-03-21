package emitter

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
)

// EmitChanges sends a json payload of cluster changes to a remote endpoint
func EmitChanges(newData interface{}, url string) {

	jsonBody, err := json.Marshal(newData)
	if err != nil {
		log.Println("Error marshalling new data", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error:", err)
	}
	defer resp.Body.Close()
}
