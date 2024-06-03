package terraform

import (
	"fmt"
	"log"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/aws/awserr"
)

// Define interfaces for the AWS services
type S3Client interface {
	CreateBucket(*s3.CreateBucketInput) (*s3.CreateBucketOutput, error)
}

type DynamoDBClient interface {
	CreateTable(*dynamodb.CreateTableInput) (*dynamodb.CreateTableOutput, error)
}

// SetupAWSBackend sets up AWS S3 and DynamoDB for Terraform state storage and locking
func SetupAWSBackend(s3Client S3Client, dynamoDBClient DynamoDBClient, backendConfig map[string]string) error {
	if err := setupS3(s3Client, backendConfig); err != nil {
		return err
	}
	if err := setupDynamoDB(dynamoDBClient, backendConfig); err != nil {
		return err
	}
	return nil
}

func setupS3(svc S3Client, backendConfig map[string]string) error {
	_, err := svc.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(backendConfig["s3"]),
	})
	if err != nil && !isBucketAlreadyOwnedByYouError(err) {
		return fmt.Errorf("failed to create S3 bucket: %v", err)
	}
	log.Printf("S3 bucket %s is ready", backendConfig["s3"])
	return nil
}

func setupDynamoDB(svc DynamoDBClient, backendConfig map[string]string) error {
	_, err := svc.CreateTable(&dynamodb.CreateTableInput{
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
		return fmt.Errorf("failed to create DynamoDB table: %v", err)
	}
	log.Printf("DynamoDB table %s is ready", backendConfig["dynamoDB"])
	return nil
}
