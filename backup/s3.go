package backup

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

func exitErrorf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
}

func (b *Backup) S3NewSession() *session.Session {
	sess, err := session.NewSession(&aws.Config{
		Credentials:      credentials.NewStaticCredentials(b.ConnInfo.AccessKey, b.ConnInfo.AccessSecret, ""),
		Region:           aws.String(b.ConnInfo.Region),
		Endpoint:         aws.String(b.ConnInfo.Endpoint),
		DisableSSL:       aws.Bool(b.ConnInfo.DisableSSL),
		S3ForcePathStyle: aws.Bool(true)},
	)
	if err != nil {
		exitErrorf("Unable to establish a connection with the S3 Provider: %v, %v", b.Provider, err)
	}
	return sess
}

func (b *Backup) S3ListObjects() map[string]time.Time {

	// Create S3 service client
	svc := s3.New(b.ConnInfo.S3Session)

	resp, err := svc.ListObjects(&s3.ListObjectsInput{Bucket: aws.String(b.ConnInfo.BucketName)})
	if err != nil {
		exitErrorf("Unable to list items in bucket %q, %v", b.ConnInfo.BucketName, err)
	}

	objects := make(map[string]time.Time)
	for _, item := range resp.Contents {
		objects[*item.Key] = *item.LastModified
	}

	return objects
}

func (b *Backup) S3GetObject() {

	file, err := os.Create("backup.tar.gz")
	if err != nil {
		exitErrorf("Unable to open file %q, %v", err)
	}

	defer file.Close()

	downloader := s3manager.NewDownloader(b.ConnInfo.S3Session)
	numBytes, err := downloader.Download(file,
		&s3.GetObjectInput{
			Bucket: aws.String(b.ConnInfo.BucketName),
			Key:    aws.String(b.FileName),
		})
	if err != nil {
		exitErrorf("Unable to download item %q, %v", b.FileName, err)
	}

	log.Println("Downloaded", file.Name(), numBytes, "bytes")
}
