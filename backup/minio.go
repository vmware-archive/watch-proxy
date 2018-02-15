package backup

import (
	"fmt"
	"os"
	"time"

	"github.com/minio/minio-go"
)

func exitMinioErrorf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}

func (b *Backup) MinioClient() *minio.Client {
	useSSL := false
	minioClient, err := minio.New(b.ConnInfo.Endpoint, b.ConnInfo.AccessKey, b.ConnInfo.AccessSecret, useSSL)
	if err != nil {
		exitMinioErrorf("Unable to connect to Minio Endpoint at:%q, %v", b.ConnInfo.Endpoint, err)
	}

	return minioClient
}

func (b *Backup) MinioListObjects() map[string]time.Time {
	doneCh := make(chan struct{})

	client := b.ConnInfo.MinioClient
	objects := make(map[string]time.Time)

	recursive := true
	objectCh := client.ListObjectsV2(b.ConnInfo.BucketName, "", recursive, doneCh)
	fmt.Println(os.Stderr, "Going into object loop")
	for object := range objectCh {
		if object.Err != nil {
			exitMinioErrorf("Received error access object: %v, %v", object, object.Err)
		}
		fmt.Fprintf(os.Stderr, "ObjectChannel: %v\n Object: %v", objectCh, object)
	}
	objects["test1"] = time.Now()

	return objects
}

func (b *Backup) MinioGetObjects() {

}
