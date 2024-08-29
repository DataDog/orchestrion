// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func AWSClientV2() {
	cfg := newCfg1()

	s3api := s3.NewFromConfig(cfg)
	res, err := s3api.CreateBucket(context.Background(), &s3.CreateBucketInput{
		Bucket: aws.String("shiny-bucket"),
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("got response: %v\n", res)
}

func newCfg1() aws.Config {
	cfg := aws.NewConfig()
	return *cfg
}

func newCfg2() aws.Config {
	cfg := &aws.Config{
		Region:       "test-region-1337",
		Credentials:  aws.AnonymousCredentials{},
		BaseEndpoint: aws.String("http://localhost:4566"),
	}
	return *cfg
}

func newCfg3() aws.Config {
	return aws.Config{
		Region:       "test-region-1337",
		Credentials:  aws.AnonymousCredentials{},
		BaseEndpoint: aws.String("http://localhost:4566"),
	}
}
