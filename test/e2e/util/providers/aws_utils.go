/*
Copyright the Velero contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package providers

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"

	"github.com/vmware-tanzu/velero/pkg/cmd/util/flag"
)

type AWSStorage string

func (s AWSStorage) ListItems(client *s3.S3, objectsV2Input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error) {
	res, err := client.ListObjectsV2(objectsV2Input)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s AWSStorage) DeleteItem(client *s3.S3, deleteObjectV2Input *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error) {
	res, err := client.DeleteObject(deleteObjectV2Input)
	if err != nil {
		return nil, err
	}
	fmt.Println(res)
	return res, nil
}
func (s AWSStorage) IsObjectsInBucket(cloudCredentialsFile, bslBucket, bslPrefix, bslConfig, backupObject string) (bool, error) {
	config := flag.NewMap()
	config.Set(bslConfig)
	region := config.Data()["region"]
	objectsInput := s3.ListObjectsV2Input{}
	objectsInput.Bucket = aws.String(bslBucket)
	objectsInput.Delimiter = aws.String("/")
	s3url := ""
	if bslPrefix != "" {
		objectsInput.Prefix = aws.String(bslPrefix)
	}
	s3Config := &aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.NewSharedCredentials(cloudCredentialsFile, ""),
	}
	if region == "minio" {
		s3url = config.Data()["s3Url"]
		s3Config = &aws.Config{
			Credentials:      credentials.NewSharedCredentials(cloudCredentialsFile, ""),
			Endpoint:         aws.String(s3url),
			Region:           aws.String(region),
			DisableSSL:       aws.Bool(true),
			S3ForcePathStyle: aws.Bool(true),
		}
	}

	sess, err := session.NewSession(s3Config)

	if err != nil {
		return false, errors.Wrapf(err, "Failed to create AWS session")
	}
	svc := s3.New(sess)

	bucketObjects, err := s.ListItems(svc, &objectsInput)
	if err != nil {
		return false, errors.Wrapf(err, "Couldn't retrieve bucket items")
	}

	for _, item := range bucketObjects.Contents {
		fmt.Println(*item)
	}
	var backupNameInStorage string
	for _, item := range bucketObjects.CommonPrefixes {
		backupNameInStorage = strings.TrimPrefix(*item.Prefix, strings.Trim(bslPrefix, "/")+"/")
		fmt.Println(backupNameInStorage)
		if strings.Contains(backupNameInStorage, backupObject) {
			fmt.Printf("Backup %s was found under prefix %s \n", backupObject, bslPrefix)
			return true, nil
		}
	}
	fmt.Printf("Backup %s was not found under prefix %s \n", backupObject, bslPrefix)
	return false, nil
}

func (s AWSStorage) DeleteObjectsInBucket(cloudCredentialsFile, bslBucket, bslPrefix, bslConfig, backupObject string) error {
	config := flag.NewMap()
	config.Set(bslConfig)
	region := config.Data()["region"]
	s3url := ""
	s3Config := &aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.NewSharedCredentials(cloudCredentialsFile, ""),
	}
	if region == "minio" {
		s3url = config.Data()["s3Url"]
		s3Config = &aws.Config{
			Credentials:      credentials.NewSharedCredentials(cloudCredentialsFile, ""),
			Endpoint:         aws.String(s3url),
			Region:           aws.String(region),
			DisableSSL:       aws.Bool(true),
			S3ForcePathStyle: aws.Bool(true),
		}
	}
	sess, err := session.NewSession(s3Config)
	if err != nil {
		return errors.Wrapf(err, "Error waiting for uploads to complete")
	}
	svc := s3.New(sess)
	fullPrefix := strings.Trim(bslPrefix, "/") + "/" + strings.Trim(backupObject, "/") + "/"
	iter := s3manager.NewDeleteListIterator(svc, &s3.ListObjectsInput{
		Bucket: aws.String(bslBucket),
		Prefix: aws.String(fullPrefix),
	})

	if err := s3manager.NewBatchDeleteWithClient(svc).Delete(aws.BackgroundContext(), iter); err != nil {
		return errors.Wrapf(err, "Error waiting for uploads to complete")
	}
	fmt.Printf("Deleted object(s) from bucket: %s %s \n", bslBucket, fullPrefix)
	return nil
}
