// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func AWSClientV1() {
	cfg := aws.NewConfig().WithRegion("us-west-2")
	sess := session.Must(session.NewSession(cfg))

	s3api := s3.New(sess)
	s3api.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String("shiny-bucket"),
	})
}
