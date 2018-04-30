// Copyright 2018 Heptio
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
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
