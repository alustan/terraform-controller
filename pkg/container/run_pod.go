package container

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// checkExistingRunPods checks for existing running, pending, or container creating pods with the specified label.
func checkExistingRunPods(clientset *kubernetes.Clientset, namespace, labelSelector string) (bool, error) {
	pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return false, err
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodRunning || pod.Status.Phase == v1.PodPending {
			return true, nil
		}
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason == "ContainerCreating" {
				return true, nil
			}
		}
	}
	return false, nil
}

// CreateRunPod creates a Kubernetes Pod that runs a script with specified environment variables and image.
func CreateRunPod(clientset *kubernetes.Clientset, name, namespace string, envVars map[string]string, scriptContent, taggedImageName, imagePullSecretName string) (string, error) {
	labelSelector := fmt.Sprintf("apprun=%s", name)

	// Check for existing pods with the same label
	exists, err := checkExistingRunPods(clientset, namespace, labelSelector)
	if err != nil {
		log.Printf("Error checking existing pods: %v", err)
		return "", err
	}

	if exists {
		log.Printf("Existing pods with label %s found, not creating new pod.", labelSelector)
		return "", fmt.Errorf("existing pods with label %s found, not creating new pod", labelSelector)
	}

	// Generate a unique pod name using the current timestamp
	timestamp := time.Now().Format("20060102150405")
	podName := fmt.Sprintf("%s-docker-run-pod-%s", name, timestamp)

	log.Printf("Creating Pod in namespace: %s with image: %s", namespace, taggedImageName)

	env := []v1.EnvVar{}
	for key, value := range envVars {
		env = append(env, v1.EnvVar{
			Name:  key,
			Value: value,
		})
		log.Printf("Setting environment variable %s=%s", key, value)
	}

	// Script to execute Terraform and capture output
	execScript := fmt.Sprintf(`
#!/bin/bash
chmod +x %s
%s
terraform output -json > /workspace/output.json
cat /workspace/output.json
`, scriptContent, scriptContent)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				"apprun": name,
			},
			Annotations: map[string]string{
				"kubectl.kubernetes.io/ttl": "3600", // TTL in seconds (1 hour)
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            "terraform",
					Image:           taggedImageName,
					ImagePullPolicy: v1.PullAlways,
					Command: []string{
						"/bin/bash",
						"-c",
						execScript,
					},
					Env: env,
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "workspace",
							MountPath: "/workspace",
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
			Volumes: []v1.Volume{
				{
					Name: "workspace",
					VolumeSource: v1.VolumeSource{
						EmptyDir: &v1.EmptyDirVolumeSource{},
					},
				},
			},
			ImagePullSecrets: []v1.LocalObjectReference{
				{
					Name: imagePullSecretName,
				},
			},
		},
	}

	log.Println("Creating the Pod...")
	_, err = clientset.CoreV1().Pods(namespace).Create(context.Background(), pod, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Failed to create Pod: %v", err)
		return "", err
	}

	log.Println("Pod created successfully.")
	return podName, nil
}


// WaitForPodCompletion waits for the pod to complete and retrieves the Terraform output
func WaitForPodCompletion(clientset *kubernetes.Clientset, namespace, podName string) (map[string]interface{}, error) {
	for {
		pod, err := clientset.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
			break
		}
		time.Sleep(5 * time.Minute)
	}

	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &v1.PodLogOptions{})
	logs, err := req.Stream(context.Background())
	if err != nil {
		return nil, err
	}
	defer logs.Close()

	logsBytes, err := io.ReadAll(logs)
	if err != nil {
		return nil, err
	}

	logsString := string(logsBytes)

	// Assuming the JSON output is printed as the last line of the logs
	lines := strings.Split(logsString, "\n")
	jsonOutput := lines[len(lines)-2]

	var terraformOutput map[string]interface{}
	err = json.Unmarshal([]byte(jsonOutput), &terraformOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Terraform output: %v", err)
	}

	return terraformOutput, nil
}

