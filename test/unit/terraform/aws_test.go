package terraform

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/stretchr/testify/assert"
)

type MockS3Client struct {
	s3Client
}

func (m *MockS3Client) CreateBucket(input *s3.CreateBucketInput) (*s3.CreateBucketOutput, error) {
	if *input.Bucket == "existing-bucket" {
		return nil, awserr.New(s3.ErrCodeBucketAlreadyOwnedByYou, "bucket already owned by you", nil)
	}
	return &s3.CreateBucketOutput{}, nil
}

type MockDynamoDBClient struct {
	dynamoDBClient
}

func (m *MockDynamoDBClient) CreateTable(input *dynamodb.CreateTableInput) (*dynamodb.CreateTableOutput, error) {
	if *input.TableName == "existing-table" {
		return nil, awserr.New(dynamodb.ErrCodeResourceInUseException, "table already exists", nil)
	}
	return &dynamodb.CreateTableOutput{}, nil
}

func TestSetupAWSBackend(t *testing.T) {
	s3Client := &MockS3Client{}
	dynamoDBClient := &MockDynamoDBClient{}

	backendConfig := map[string]string{
		"s3":       "test-bucket",
		"dynamoDB": "test-table",
	}

	err := SetupAWSBackend(s3Client, dynamoDBClient, backendConfig)
	assert.NoError(t, err)
}
