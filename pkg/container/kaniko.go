package container

import (
	"context"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	
	"k8s.io/client-go/kubernetes"
)



// checkExistingPods checks for existing pods with the specified label.
func checkExistingBuildPods(clientset *kubernetes.Clientset, namespace, labelSelector string) (bool, error) {
	pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return false, err
	}

	return len(pods.Items) > 0, nil
}

// CreateBuildPod creates a Kubernetes Pod to run a Kaniko build
func CreateBuildPod(clientset *kubernetes.Clientset, name, namespace, configMapName, imageName, pvcName, dockerSecretName, repoDir string) (string, error) {
	err := EnsurePVC(clientset, namespace, pvcName)
	if err != nil {
		log.Printf("Failed to ensure PVC: %v", err)
		return "", err
	}

	// Generate a unique pod name using the current timestamp
	timestamp := time.Now().Format("20060102150405")
	podName := fmt.Sprintf("%s-docker-build-pod-%s", name, timestamp)
	labelSelector := fmt.Sprintf("app-build=%s", name)
	
	// Generate a unique tag using the current timestamp
	taggedImageName := fmt.Sprintf("%s:%s", imageName, timestamp)

	// Check for existing pods with the same label
	exists, err := checkExistingBuildPods(clientset, namespace, labelSelector)
	if err != nil {
		log.Printf("Error checking existing pods: %v", err)
		return "", err
	}

	if exists {
		log.Printf("Existing pods with label %s found, not creating new pod.", labelSelector)
		return "", fmt.Errorf("existing build pod already running")
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				"app": name,
			},
			Annotations: map[string]string{
				"kubectl.kubernetes.io/ttl": "3600", // TTL in seconds (1 hour)
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "kaniko",
					Image: "gcr.io/kaniko-project/executor:v1.23.1-debug",
					Args: []string{
						"--dockerfile=/config/Dockerfile",
						"--destination=" + taggedImageName,
						"--context=/workspace",
					},
					Env: []corev1.EnvVar{
						{
							Name:  "DOCKER_CONFIG",
							Value: "/root/.docker",
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "dockerfile-config",
							MountPath: "/config",
						},
						{
							Name:      "workspace",
							MountPath: "/workspace",
						},
						{
							Name:      "docker-credentials",
							MountPath: "/root/.docker",
						},
						{
							Name:      "kaniko-logs",
							MountPath: "/logs",
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
			Volumes: []corev1.Volume{
				{
					Name: "dockerfile-config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: configMapName,
							},
							Items: []corev1.KeyToPath{
								{
									Key:  "Dockerfile",
									Path: "Dockerfile",
								},
							},
						},
					},
				},
				{
					Name: "workspace",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: repoDir, // Host path to the cloned repository
						},
					},
				},
				{
					Name: "docker-credentials",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: dockerSecretName,
							Items: []corev1.KeyToPath{
								{
									Key:  ".dockerconfigjson",
									Path: "config.json",
								},
							},
						},
					},
				},
				{
					Name: "kaniko-logs",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
		},
	}

	// Create the pod
	_, err = clientset.CoreV1().Pods(namespace).Create(context.Background(), pod, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Failed to create Pod: %v", err)
		return "", err
	}

	log.Printf("Created Pod: %s", podName)
	log.Printf("Image will be pushed with tag: %s", taggedImageName)
	return taggedImageName, nil
}
