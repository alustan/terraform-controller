

package terraform_test

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"controller/pkg/terraform"
)

// MockS3Client is a mock for the S3 client
type MockS3Client struct {
	s3iface.S3API
	mock.Mock
}

func (m *MockS3Client) CreateBucket(input *s3.CreateBucketInput) (*s3.CreateBucketOutput, error) {
	args := m.Called(input)
	return nil, args.Error(1)
}

// MockDynamoDBClient is a mock for the DynamoDB client
type MockDynamoDBClient struct {
	dynamodbiface.DynamoDBAPI
	mock.Mock
}

func (m *MockDynamoDBClient) CreateTable(input *dynamodb.CreateTableInput) (*dynamodb.CreateTableOutput, error) {
	args := m.Called(input)
	return nil, args.Error(1)
}

func TestSetupAWSBackend(t *testing.T) {
    backendConfig := map[string]string{
        "region":   "us-west-2",
        "s3":       "test-bucket",
        "dynamoDB": "test-table",
    }

    // Mock S3 client
    mockS3 := new(MockS3Client)
    mockS3.On("CreateBucket", mock.Anything).Return(nil, nil)

    // Mock DynamoDB client
    mockDynamoDB := new(MockDynamoDBClient)
    mockDynamoDB.On("CreateTable", mock.Anything).Return(nil, nil)

    // Set the mocked clients
    terraform.SetupS3Func(mockS3, backendConfig)
    terraform.SetupDynamoDBFunc(mockDynamoDB, backendConfig)

    err := terraform.SetupAWSBackend(backendConfig)
    assert.NoError(t, err)

    mockS3.AssertExpectations(t)
    mockDynamoDB.AssertExpectations(t)
}


func TestSetupS3(t *testing.T) {
	backendConfig := map[string]string{
		"s3": "test-bucket",
	}

	t.Run("create bucket successfully", func(t *testing.T) {
		mockS3 := new(MockS3Client)
		mockS3.On("CreateBucket", mock.Anything).Return(nil, nil)

		err := terraform.SetupS3WithClient(mockS3, backendConfig)
		assert.NoError(t, err)
		mockS3.AssertExpectations(t)
	})

	t.Run("bucket already owned by you", func(t *testing.T) {
		mockS3 := new(MockS3Client)
		mockS3.On("CreateBucket", mock.Anything).Return(nil, awserr.New(s3.ErrCodeBucketAlreadyOwnedByYou, "bucket already owned by you", nil))

		err := terraform.SetupS3WithClient(mockS3, backendConfig)
		assert.NoError(t, err)
		mockS3.AssertExpectations(t)
	})

	t.Run("create bucket failed", func(t *testing.T) {
		mockS3 := new(MockS3Client)
		mockS3.On("CreateBucket", mock.Anything).Return(nil, errors.New("failed to create bucket"))

		err := terraform.SetupS3WithClient(mockS3, backendConfig)
		assert.Error(t, err)
		mockS3.AssertExpectations(t)
	})
}

func TestSetupDynamoDB(t *testing.T) {
	backendConfig := map[string]string{
		"dynamoDB": "test-table",
	}

	t.Run("create table successfully", func(t *testing.T) {
		mockDynamoDB := new(MockDynamoDBClient)
		mockDynamoDB.On("CreateTable", mock.Anything).Return(nil, nil)

		err := terraform.SetupDynamoDBWithClient(mockDynamoDB, backendConfig)
		assert.NoError(t, err)
		mockDynamoDB.AssertExpectations(t)
	})

	t.Run("table already exists", func(t *testing.T) {
		mockDynamoDB := new(MockDynamoDBClient)
		mockDynamoDB.On("CreateTable", mock.Anything).Return(nil, awserr.New(dynamodb.ErrCodeResourceInUseException, "table already exists", nil))

		err := terraform.SetupDynamoDBWithClient(mockDynamoDB, backendConfig)
		assert.NoError(t, err)
		mockDynamoDB.AssertExpectations(t)
	})

	t.Run("create table failed", func(t *testing.T) {
		mockDynamoDB := new(MockDynamoDBClient)
		mockDynamoDB.On("CreateTable", mock.Anything).Return(nil, errors.New("failed to create table"))

		err := terraform.SetupDynamoDBWithClient(mockDynamoDB, backendConfig)
		assert.Error(t, err)
		mockDynamoDB.AssertExpectations(t)
	})
}
