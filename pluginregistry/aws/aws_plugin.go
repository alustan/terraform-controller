package aws

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"k8s.io/client-go/kubernetes"
)
type AWSPlugin struct {
	clientset *kubernetes.Clientset
	workspace string
	region    string
}

func NewAWSPlugin(clientset *kubernetes.Clientset, workspace, region string) *AWSPlugin {
	return &AWSPlugin{
		clientset: clientset,
		workspace: workspace,
		region:    region,
	}
}

func init() {
	// We don't register the plugin here, because we need to pass the clientset, workspace, and region dynamically
}

func (p *AWSPlugin) GetDockerfileAdditions() string {
	return `RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" && \
            apt install unzip && \
            unzip awscliv2.zip && \
            ./aws/install && \
            rm -rf awscliv2.zip aws`
}

func (p *AWSPlugin) Execute() (map[string]interface{}, error) {
	creds, err := RetrieveCreds(p.clientset, p.workspace, p.region)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	err = json.Unmarshal([]byte(creds), &result)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling creds: %v", err)
	}
	return result, nil
}

func RetrieveCreds(clientset *kubernetes.Clientset, workspace, region string) (string, error) {
	secretName := fmt.Sprintf("argocd-%s", workspace)
	secretValue, err := RetrieveSecret(secretName, region)
	if err != nil {
		return "", fmt.Errorf("error retrieving ArgoCD secret: %v", err)
	}

	status := make(map[string]interface{})
	status["argocdUsername"] = "admin"
	status["argocdPassword"] = secretValue

	statusJSON, err := json.Marshal(status)
	if err != nil {
		return "", fmt.Errorf("error marshalling status to JSON: %v", err)
	}

	return string(statusJSON), nil
}

func RetrieveSecret(secretName, region string) (string, error) {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(region),
	}))
	svc := secretsmanager.New(sess)
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	}
	result, err := svc.GetSecretValue(input)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve secret: %v", err)
	}
	return *result.SecretString, nil
}
