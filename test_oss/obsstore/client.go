package obsstore

import (
	"fmt"
	"io"
	"sort"

	obs "github.com/huaweicloud/huaweicloud-sdk-go-obs/obs"
)

const Bucket = "skilljudge"

var client *obs.ObsClient

func Init(ak, sk, endpoint string) error {
	var err error
	client, err = obs.New(ak, sk, endpoint)
	return err
}

// InitiateMultipartUpload creates a new multipart upload and returns the OBS UploadId.
func InitiateMultipartUpload(objectKey string) (string, error) {
	input := &obs.InitiateMultipartUploadInput{}
	input.Bucket = Bucket
	input.Key = objectKey
	output, err := client.InitiateMultipartUpload(input)
	if err != nil {
		return "", fmt.Errorf("InitiateMultipartUpload: %w", err)
	}
	return output.UploadId, nil
}

// UploadPart uploads one chunk and returns the ETag.
func UploadPart(objectKey, uploadID string, partNumber int, body io.Reader, size int64) (string, error) {
	input := &obs.UploadPartInput{}
	input.Bucket = Bucket
	input.Key = objectKey
	input.UploadId = uploadID
	input.PartNumber = partNumber
	input.Body = body
	input.PartSize = size
	output, err := client.UploadPart(input)
	if err != nil {
		return "", fmt.Errorf("UploadPart %d: %w", partNumber, err)
	}
	return output.ETag, nil
}

// CompleteMultipartUpload finalizes the multipart upload and returns the public URL.
func CompleteMultipartUpload(objectKey, uploadID string, parts []obs.Part) (string, error) {
	// OBS requires parts sorted by PartNumber
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})

	input := &obs.CompleteMultipartUploadInput{}
	input.Bucket = Bucket
	input.Key = objectKey
	input.UploadId = uploadID
	input.Parts = parts
	_, err := client.CompleteMultipartUpload(input)
	if err != nil {
		return "", fmt.Errorf("CompleteMultipartUpload: %w", err)
	}
	url := fmt.Sprintf("https://%s.obs.cn-east-3.myhuaweicloud.com/%s", Bucket, objectKey)
	return url, nil
}

// AbortMultipartUpload cancels an in-progress multipart upload.
func AbortMultipartUpload(objectKey, uploadID string) error {
	input := &obs.AbortMultipartUploadInput{}
	input.Bucket = Bucket
	input.Key = objectKey
	input.UploadId = uploadID
	_, err := client.AbortMultipartUpload(input)
	return err
}
