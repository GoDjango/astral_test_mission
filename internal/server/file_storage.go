package server

import (
	"context"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"net/url"
)

type FileStorage struct {
	client *minio.Client
}

func NewFileStorage(minioDSN string) (*FileStorage, error) {
	minioUrl, err := url.Parse(minioDSN)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse minio url. Error: %s ", err)
	}
	secretKey, _ := minioUrl.User.Password()

	client, err := minio.New(minioUrl.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(minioUrl.User.Username(), secretKey, ""),
		Secure: minioUrl.Scheme == "https",
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to create minio client. Error: %s ", err)
	}

	exists, err := client.BucketExists(context.Background(), MinioBucketName)
	if err != nil {
		return nil, fmt.Errorf("Failed to check bucket exists ")
	}
	if !exists {
		err = client.MakeBucket(context.Background(), MinioBucketName, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("Failed to create bucket ")
		}
	}

	return &FileStorage{
		client: client,
	}, nil
}
