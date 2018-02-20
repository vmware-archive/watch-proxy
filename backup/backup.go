package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
)

type Backup struct {
	Provider string
	FileName string
	ConnInfo ConnectionInfo
}

type ConnectionInfo struct {
	Region       string
	S3Session    *session.Session
	BucketName   string
	Endpoint     string
	AccessKey    string
	AccessSecret string
	DisableSSL   bool
}

func (b *Backup) NewSession() {

	b.ConnInfo.S3Session = b.S3NewSession()
}

func (b *Backup) List() {

	// just s3 for the moment
	objects := make(map[string]time.Time)
	//fmt.Fprintf(os.Stderr, "What provider are we: %v", b.Provider)
	switch b.Provider {
	case "aws":
		b.ConnInfo.DisableSSL = false
	case "minio":
		b.ConnInfo.DisableSSL = true
	}
	objects = b.S3ListObjects()

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

func (b *Backup) Get() (string, error) {

	tarBuf := b.S3GetObject()

	unpackDir, err := unPack(tarBuf)
	if err != nil {
		log.Println("Could not unpack file:", err)
	}
	return unpackDir, err
}

// Untar takes a destination path and a reader; a tar reader loops over the tarfile
// creating the file structure at 'dst' along the way, and writing any files
func unPack(src []byte) (string, error) {

	unpackDir, err := ioutil.TempDir("/tmp", "clerk")
	if err != nil {
		fmt.Printf("Could not create a temporary directory: %v", err)
		os.Exit(1)
	}

	byteReader := bytes.NewReader(src)
	gzf, err := gzip.NewReader(byteReader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not read from GZIP file: %v", err)
		os.Exit(1)
	}
	tarReader := tar.NewReader(gzf)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		destination := filepath.Join(unpackDir, header.Name)

		// Got this part from https://medium.com/@skdomino/taring-untaring-files-in-go-6b07cf56bc07
		switch header.Typeflag {

		case tar.TypeDir:
			if _, err := os.Stat(destination); err != nil {
				if err := os.MkdirAll(destination, 0755); err != nil {
					return "", err
				}
			}

		case tar.TypeReg:
			outDir, _ := filepath.Split(destination)
			if _, err := os.Stat(destination); err != nil {
				if err := os.MkdirAll(outDir, 0755); err != nil {
					return "", err
				}
			}
			f, err := os.OpenFile(destination, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return "", err
			}
			defer f.Close()

			if _, err := io.Copy(f, tarReader); err != nil {
				return "", err
			}
		}
	}
	return unpackDir, nil
}
