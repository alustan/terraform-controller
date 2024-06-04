package terraform

import (
	"log"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"controller/pkg/util"
)

// Function to extract the content of the script
func ExtractScriptContent(clientset *kubernetes.Clientset, namespace string, script map[string]interface{}) (string, error) {
	// Check if the script is defined inline
	inlineScript, isInline := script["inline"].(string)
	if isInline {
		return inlineScript, nil
	}

	// If not inline, check if it's defined via a ConfigMap reference
	configMapRef, isConfigMapRef := script["configMapRef"].(map[string]interface{})
	if !isConfigMapRef {
		return "", logErrorf("script is not defined inline or via a ConfigMap reference")
	}

	// Retrieve the name and key of the ConfigMap
	name, isName := configMapRef["name"].(string)
	key, isKey := configMapRef["key"].(string)
	if !isName || !isKey {
		return "", logErrorf("missing name or key in ConfigMap reference")
	}

	// For demonstration purposes, let's assume we have a Kubernetes client
	// that can retrieve the content of the ConfigMap based on its name and key
	configMapContent, err := util.GetConfigMapContent(clientset ,namespace, name, key)
	if err != nil {
		return "", err
	}

	return configMapContent, nil
}

// logErrorf logs the error and returns it
func logErrorf(format string, args ...interface{}) error {
	err := fmt.Errorf(format, args...)
	log.Println(err)
	return err
}
