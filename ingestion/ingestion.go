package ingestion

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

func findType(file []byte) string {
	var resource map[string]interface{}
	if err := json.Unmarshal(file, &resource); err != nil {
		log.Println("Error unmarshaling file data", err)
	}
	return resource["kind"].(string)
}

func ReadFiles(dir string) map[string][][]byte {

	files := []string{}
	err := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		files = append(files, path)
		return nil
	})
	if err != nil {
		log.Println("Error walking file path", err)
	}

	filesAndTypes := make(map[string][][]byte)
	var resourceFiles [][]byte
	var reseourceType string

	// var wg sync.WaitGroup
	// wg.Add(len(files))

	for _, file := range files {
		fileInfo, _ := os.Stat(file)
		if fileInfo.IsDir() {
			continue
		}
		// go func(wg *sync.WaitGroup) {
		fileContents, err := ioutil.ReadFile(file)
		if err != nil {
			log.Println("Error opening file", err)
		}
		reseourceType = findType(fileContents)
		resourceFiles = append(resourceFiles, fileContents)
		// wg.Done()
		// }(&wg)

	}
	filesAndTypes[reseourceType] = resourceFiles
	return filesAndTypes

}
