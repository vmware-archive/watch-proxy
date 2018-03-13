package config

import (
	"log"
	"os"
	"time"
)

type FileWatcher struct {
}

// New creates new file watcher
func (fw FileWatcher) New(ch chan bool, file string) {
	fileWatcher(ch, file)
}

func fileWatcher(changed chan bool, file string) {
	go func() {
		// create start time of now so it always loads the first time
		startTime := time.Now()
		for {
			info, err := os.Stat(file)
			if err != nil {
				log.Println("Error accessing file", err)
			}

			modTime := info.ModTime()
			if startTime != modTime {
				// reset the start time to our newly seen modified time
				startTime = modTime
				changed <- true
			}
			// sleep for some time we we aren't always banging
			// on the file system
			time.Sleep(time.Second * 3)
		}
	}()

}

// func UpdateConfig(config *map[string]bool) {
// 	file, err := os.Open("/tmp/labeler.config")
// 	if err != nil {
// 		log.Fatal("Error accessing file", err)
// 	}
// 	defer file.Close()

// 	intMap := make(map[string]bool)
// 	scanner := bufio.NewScanner(file)
// 	for scanner.Scan() {
// 		// fmt.Println(scanner.Text())
// 		intMap[scanner.Text()] = true
// 	}

// 	if err := scanner.Err(); err != nil {
// 		log.Fatal("Error reading from file", err)
// 	}

// 	*config = intMap
// }
