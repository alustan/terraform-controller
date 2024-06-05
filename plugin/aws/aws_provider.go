package plugin

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
    "controller/plugin"
)

type AWSProvider struct{}

func init() {
    plugin.RegisterProvider("aws", &AWSProvider{})
}

func (p *AWSProvider) SetupBackend(backendConfig map[string]string) error {
    sess, err := session.NewSession(&aws.Config{
        Region: aws.String(backendConfig["region"]),
    })
    if err != nil {
        return logErrorAndReturn("failed to create AWS session: %v", err)
    }

    // Set up S3 for state storage
    if err := p.SetupS3WithClient(s3.New(sess), backendConfig); err != nil {
        return err
    }

    // Set up DynamoDB for state locking
    if err := p.SetupDynamoDBWithClient(dynamodb.New(sess), backendConfig); err != nil {
        return err
    }

    return nil
}

func (p *AWSProvider) GetDockerfileAdditions() string {
    return `RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" && \
            apt install unzip && \
            unzip awscliv2.zip && \
            ./aws/install && \
            rm -rf awscliv2.zip aws`
}

func (p *AWSProvider) SetupS3WithClient(s3Client s3iface.S3API, backendConfig map[string]string) error {
    _, err := s3Client.CreateBucket(&s3.CreateBucketInput{
        Bucket: aws.String(backendConfig["s3"]),
    })
    if err != nil && !isBucketAlreadyOwnedByYouError(err) {
        return logErrorAndReturn("failed to create S3 bucket: %v", err)
    }
    log.Printf("S3 bucket %s is ready", backendConfig["s3"])
    return nil
}

func (p *AWSProvider) SetupDynamoDBWithClient(dynamoDBClient dynamodbiface.DynamoDBAPI, backendConfig map[string]string) error {
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

