package fsyncperf

import (
	"bytes"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/credentials"
	"github.com/aws/aws-sdk-go-v2/aws/session"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func UploadFile(uploadFileDir string) error {
	region := os.Getenv("AWS_REGION")
	bucket := os.Getenv("AWS_BUCKET")
	access_key := os.Getenv("ACCESS_KEY")
	secret_key := os.Getenv("SECRET_KEY")
	session, err := session.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(access_key, secret_key, ""),
		Region:      aws.String(region),
	})
	if err != nil {
		return err
	}

	err = uploadFileToS3(session, uploadFileDir, bucket)
	if err != nil {
		return err
	}

	return nil
}

func uploadFileToS3(session *session.Session, uploadFileDir, bucket string) error {
	upFile, err := os.Open(uploadFileDir)
	if err != nil {
		return err
	}
	defer upFile.Close()

	upFileInfo, _ := upFile.Stat()
	var fileSize int64 = upFileInfo.Size()
	fileBuffer := make([]byte, fileSize)
	upFile.Read(fileBuffer)

	_, err = s3.New(session).PutObject(&s3.PutObjectInput{
		Bucket:               aws.String(bucket),
		Key:                  aws.String(uploadFileDir),
		ACL:                  aws.String("private"),
		Body:                 bytes.NewReader(fileBuffer),
		ContentLength:        aws.Int64(fileSize),
		ContentType:          aws.String(http.DetectContentType(fileBuffer)),
		ContentDisposition:   aws.String("attachment"),
		ServerSideEncryption: aws.String("AES256"),
	})
	if err != nil {
		return err
	}

	return nil
}
