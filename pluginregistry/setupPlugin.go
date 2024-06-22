package pluginregistry

import (
	"github.com/alustan/terraform-controller/pluginregistry/aws"
	"k8s.io/client-go/kubernetes"	
)



func SetupPlugin(clientset *kubernetes.Clientset,providerType, workspace, region string) (Plugin, error) {
	switch providerType {
	case "aws":
		awsPlugin := aws.NewAWSPlugin(clientset, workspace, region)
		RegisterPlugin("aws", awsPlugin)
		return awsPlugin, nil
	default:
		plugin, err := GetPlugin(providerType)
		if err != nil {
			return nil, err
		}
		return plugin, nil
	}
}
