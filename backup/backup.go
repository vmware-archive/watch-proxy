package backup

import (
	"log"
	"regexp"
	"time"

	"github.com/mholt/archiver"
	"github.com/minio/minio-go"
)

type Backup struct {
	Provider string
	FileName string
	ConnInfo ConnectionInfo
}

type ConnectionInfo struct {
	Region       string
	BucketName   string
	Endpoint     string
	AccessKey    string
	AccessSecret string
	MinioClient  *minio.Client
}

func (b *Backup) List() {
	// just s3 for the moment
	objects := make(map[string]time.Time)
	switch b.Provider {
	case "aws":
		objects = b.S3ListObjects()
	case "minio":
		b.ConnInfo.MinioClient = b.MinioClient()
		objects = b.MinioListObjects()
	}

	b.newestBackup(objects)

}

func (b *Backup) newestBackup(objects map[string]time.Time) {
	// loop through map making sure key name fits
	// our pattern match
	newest := time.Time{}
	var backupFile string
	for k, v := range objects {
		if ok, _ := regexp.MatchString(".*.tar.gz", k); ok == false {
			continue
		}
		if v.After(newest) {
			newest = v
			backupFile = k
		}
	}

	// after looping save the newest file name
	b.FileName = backupFile
}

func (b *Backup) Get() {

	saveFileName := "backup.tar.gz"
	// just s3 for the moment
	b.S3GetObject()

	err := unPack(saveFileName, ".")
	if err != nil {
		log.Println("Could not unpack file:", err)
	}
}

// Untar takes a destination path and a reader; a tar reader loops over the tarfile
// creating the file structure at 'dst' along the way, and writing any files
func unPack(src, dst string) error {
	err := archiver.TarGz.Open(src, dst)
	return err
}
