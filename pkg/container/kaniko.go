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

// checkExistingBuildPods checks for existing running, pending, or container creating pods with the specified label.
func checkExistingBuildPods(clientset *kubernetes.Clientset, namespace, labelSelector string) (bool, error) {
	pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return false, err
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodPending {
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


// CreateBuildPod creates a Kubernetes Pod to run a Kaniko build.
func CreateBuildPod(clientset *kubernetes.Clientset, name, namespace, configMapName, imageName, pvcName, dockerSecretName, repoDir string) (string, string, error) {
	err := EnsurePVC(clientset, namespace, pvcName)
	if err != nil {
		log.Printf("Failed to ensure PVC: %v", err)
		return "", "", err
	}

	labelSelector := fmt.Sprintf("appbuild=%s", name)

	// Check for existing pods with the same label
	exists, err := checkExistingBuildPods(clientset, namespace, labelSelector)
	if err != nil {
		log.Printf("Error checking existing pods: %v", err)
		return "", "", err
	}

	if exists {
		log.Printf("Existing pods with label %s found, not creating new pod.", labelSelector)
		return "", "", fmt.Errorf("existing build pod already running")
	}

	// Generate a unique pod name using the current timestamp
	timestamp := time.Now().Format("20060102150405")
	podName := fmt.Sprintf("%s-docker-build-pod-%s", name, timestamp)

	// Generate a unique tag using the current timestamp
	taggedImageName := fmt.Sprintf("%s:%s", imageName, timestamp)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				"appbuild": name,
			},
			Annotations: map[string]string{
				"kubectl.kubernetes.io/ttl": "1800", // TTL in seconds (30 mins)
			},
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name:  "copy-repo",
					Image: "busybox",
					Command: []string{
						"sh", "-c", fmt.Sprintf("cp -r %s/. /workspace/ && ls /workspace/", repoDir),
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "host-repo",
							MountPath: repoDir,
						},
						{
							Name:      "workspace",
							MountPath: "/workspace",
						},
					},
				},
			},
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
						EmptyDir: &corev1.EmptyDirVolumeSource{},
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
				{
					Name: "host-repo",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: repoDir,
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
		return "", "", err
	}

	log.Printf("Created Pod: %s", podName)
	log.Printf("Image will be pushed with tag: %s", taggedImageName)
	return taggedImageName, podName, nil
}