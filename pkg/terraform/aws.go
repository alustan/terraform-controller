package terraform

import (
	"log"
     "fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/s3"
)

// SetupAWSBackend sets up AWS S3 and DynamoDB for Terraform state storage and locking
func SetupAWSBackend(backendConfig map[string]string) error {
	// Initialize a session in the specified region
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(backendConfig["region"]),
	})
	if err != nil {
		return logErrorAndReturn("failed to create AWS session: %v", err)
	}

	// Set up S3 for state storage
	if err := setupS3(sess, backendConfig); err != nil {
		return err
	}

	// Set up DynamoDB for state locking
	if err := setupDynamoDB(sess, backendConfig); err != nil {
		return err
	}

	return nil
}

// setupS3 creates the S3 bucket if it does not exist
func setupS3(sess *session.Session, backendConfig map[string]string) error {
	svc := s3.New(sess)
	_, err := svc.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(backendConfig["s3"]),
	})
	if err != nil && !isBucketAlreadyOwnedByYouError(err) {
		return logErrorAndReturn("failed to create S3 bucket: %v", err)
	}
	log.Printf("S3 bucket %s is ready", backendConfig["s3"])
	return nil
}

// setupDynamoDB creates the DynamoDB table if it does not exist
func setupDynamoDB(sess *session.Session, backendConfig map[string]string) error {
	svc := dynamodb.New(sess)
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
		return logErrorAndReturn("failed to create DynamoDB table: %v", err)
	}
	log.Printf("DynamoDB table %s is ready", backendConfig["dynamoDB"])
	return nil
}

// isBucketAlreadyOwnedByYouError checks if the error is due to the bucket already being owned by the user
func isBucketAlreadyOwnedByYouError(err error) bool {
	if aerr, ok := err.(awserr.Error); ok && aerr.Code() == s3.ErrCodeBucketAlreadyOwnedByYou {
		return true
	}
	return false
}

// isTableAlreadyExistsError checks if the error is due to the table already existing
func isTableAlreadyExistsError(err error) bool {
	if aerr, ok := err.(awserr.Error); ok && aerr.Code() == dynamodb.ErrCodeResourceInUseException {
		return true
	}
	return false
}

// logErrorAndReturn logs the error and returns it
func logErrorAndReturn(format string, args ...interface{}) error {
	err := fmt.Errorf(format, args...)
	log.Println(err)
	return err
}
