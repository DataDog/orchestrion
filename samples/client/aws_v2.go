package main

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func AWSClientV2() {
	cfg := aws.Config{
		Region:       "test-region-1337",
		Credentials:  aws.AnonymousCredentials{},
		BaseEndpoint: aws.String("http://localhost:4566"),
	}

	s3api := s3.NewFromConfig(cfg)
	res, err := s3api.CreateBucket(context.Background(), &s3.CreateBucketInput{
		Bucket: aws.String("shiny-bucket"),
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("got response: %v\n", res)
}
