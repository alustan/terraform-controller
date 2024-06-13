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
func CreateBuildPod(clientset *kubernetes.Clientset, name, namespace, configMapName, imageName, dockerSecretName, repoDir, gitRepo, branch, sshKey string) (string, string, error) {

	labelSelector := fmt.Sprintf("appbuild=%s", name)
	pvcName := fmt.Sprintf("pvc-%s", name)

	err := EnsurePVC(clientset, namespace, pvcName)
	if err != nil {
		log.Printf("Error creating PVC: %v", err)
		return "", "", err
	}

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
					Name:  "setup-repo-dir",
					Image: "busybox",
					Command: []string{
						"sh", "-c", fmt.Sprintf("mkdir -p %s && chmod 777 %s", repoDir, repoDir),
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "workspace",
							MountPath: repoDir,
						},
					},
				},
				{
					Name:  "git-clone",
					Image: "docker.io/alustan/git-clone:0.3.0",
					Env: []corev1.EnvVar{
						{
							Name:  "REPO_URL",
							Value: gitRepo,
						},
						{
							Name:  "BRANCH",
							Value: branch,
						},
						{
							Name:  "REPO_DIR",
							Value: repoDir,
						},
						{
							Name:  "SSH_KEY",
							Value: sshKey,
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "workspace",
							MountPath: repoDir,
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
						"--context=" + repoDir,
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
							MountPath: repoDir,
						},
						{
							Name:      "docker-credentials",
							MountPath: "/root/.docker",
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
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
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
