package terraform

import (
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
)

// SetupAWSBackend sets up AWS S3 and DynamoDB for Terraform state storage and locking
func SetupAWSBackend(backendConfig map[string]string) error {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(backendConfig["region"]),
	})
	if err != nil {
		return logErrorAndReturn("failed to create AWS session: %v", err)
	}

	// Set up S3 for state storage
	if err := SetupS3WithClient(s3.New(sess), backendConfig); err != nil {
		return err
	}

	// Set up DynamoDB for state locking
	if err := SetupDynamoDBWithClient(dynamodb.New(sess), backendConfig); err != nil {
		return err
	}

	return nil
}

// SetupS3 creates the S3 bucket if it does not exist
func SetupS3(sess *session.Session, backendConfig map[string]string) error {
	s3Client := s3.New(sess)
	return SetupS3WithClient(s3Client, backendConfig)
}

// SetupS3WithClient creates the S3 bucket if it does not exist, using a provided S3 client
func SetupS3WithClient(s3Client s3iface.S3API, backendConfig map[string]string) error {
	_, err := s3Client.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(backendConfig["s3"]),
	})
	if err != nil && !isBucketAlreadyOwnedByYouError(err) {
		return logErrorAndReturn("failed to create S3 bucket: %v", err)
	}
	log.Printf("S3 bucket %s is ready", backendConfig["s3"])
	return nil
}

// SetupDynamoDB creates the DynamoDB table if it does not exist
func SetupDynamoDB(sess *session.Session, backendConfig map[string]string) error {
	dynamoDBClient := dynamodb.New(sess)
	return SetupDynamoDBWithClient(dynamoDBClient, backendConfig)
}

// SetupDynamoDBWithClient creates the DynamoDB table if it does not exist, using a provided DynamoDB client
func SetupDynamoDBWithClient(dynamoDBClient dynamodbiface.DynamoDBAPI, backendConfig map[string]string) error {
	_, err := dynamoDBClient.CreateTable(&dynamodb.CreateTableInput{
		TableName: aws.String(backendConfig["dynamoDB"]),
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String("LockID"),
				KeyType:       aws.String("HASH"),
			},
		},
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String("LockID"),
				AttributeType: aws.String("S"),
			},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(1),
			WriteCapacityUnits: aws.Int64(1),
		},
	})
	if err != nil && !isTableAlreadyExistsError(err) {
		return logErrorAndReturn("failed to create DynamoDB table: %v", err)
	}
	log.Printf("DynamoDB table %s is ready", backendConfig["dynamoDB"])
	return nil
}

func isBucketAlreadyOwnedByYouError(err error) bool {
	if aerr, ok := err.(awserr.Error); ok && aerr.Code() == s3.ErrCodeBucketAlreadyOwnedByYou {
		return true
	}
	return false
}

func isTableAlreadyExistsError(err error) bool {
	if aerr, ok := err.(awserr.Error); ok && aerr.Code() == dynamodb.ErrCodeResourceInUseException {
		return true
	}
	return false
}

func logErrorAndReturn(format string, args ...interface{}) error {
	err := fmt.Errorf(format, args...)
	log.Println(err)
	return err
}

// Dependency injection variables
var (
	SetupS3Func       = SetupS3WithClient
	SetupDynamoDBFunc = SetupDynamoDBWithClient
)
