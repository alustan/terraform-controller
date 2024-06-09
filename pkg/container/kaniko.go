package container

import (
	"context"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// removeFinalizers removes all finalizers from the Pod
func removeFinalizers(clientset *kubernetes.Clientset, namespace, podName string) error {
	patch := []byte(`{"metadata":{"finalizers":[]}}`)
	_, err := clientset.CoreV1().Pods(namespace).Patch(context.Background(), podName, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		log.Printf("Failed to remove finalizers from Pod: %v", err)
	}
	return err
}

// deletePodWithRetry attempts to delete a Pod with retry logic
func deletePodWithRetry(clientset *kubernetes.Clientset, namespace, podName string) error {
	maxAttempts := 5
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := clientset.CoreV1().Pods(namespace).Delete(context.Background(), podName, metav1.DeleteOptions{})
		if err == nil {
			log.Printf("Deleted existing Pod: %s", podName)
			return nil
		}
		if apierrors.IsNotFound(err) {
			log.Printf("Pod %s not found in namespace %s", podName, namespace)
			return nil
		}
		log.Printf("Attempt %d: Failed to delete Pod: %v", attempt, err)
		if attempt < maxAttempts {
			time.Sleep(1 * time.Minute)
		} else {
			log.Printf("Max attempts reached. Giving up on deleting Pod: %s", podName)
			return err
		}
	}
	return fmt.Errorf("failed to delete Pod %s after %d attempts", podName, maxAttempts)
}

// WaitForPodDeletion waits until the specified pod is deleted
func WaitForPodDeletion(clientset *kubernetes.Clientset, namespace, podName string) error {
	for {
		_, err := clientset.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			log.Printf("Pod %s is confirmed deleted", podName)
			return nil
		}
		if err != nil {
			log.Printf("Error getting Pod: %v", err)
			return err
		}
		log.Printf("Pod %s is still being deleted. Waiting...", podName)
		time.Sleep(5 * time.Second)
	}
}

// CreateBuildPod creates a Kubernetes Pod to run a Kaniko build
func CreateBuildPod(clientset *kubernetes.Clientset, name, namespace, configMapName, imageName, pvcName, dockerSecretName, repoDir string) (string, error) {
	err := EnsurePVC(clientset, namespace, pvcName)
	if err != nil {
		log.Printf("Failed to ensure PVC: %v", err)
		return "", err
	}

	podName := fmt.Sprintf("%s-docker-build-pod", name)

	// Attempt to get the existing pod
	pod, err := clientset.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
	if err == nil {
		// Pod exists, attempt to remove finalizers
		if len(pod.ObjectMeta.Finalizers) > 0 {
			log.Printf("Removing finalizers from Pod: %s", podName)
			err := removeFinalizers(clientset, namespace, podName)
			if err != nil {
				return "", err
			}
		}

		// Delete the pod with retry logic
		err = deletePodWithRetry(clientset, namespace, podName)
		if err != nil {
			return "", err
		}

		// Wait for pod deletion to complete
		err = WaitForPodDeletion(clientset, namespace, podName)
		if err != nil {
			return "", err
		}
	} else if !apierrors.IsNotFound(err) {
		log.Printf("Error checking for existing Pod: %v", err)
		return "", err
	} else {
		log.Printf("No existing Pod to delete: %s", podName)
	}

	// Generate a unique tag using the current timestamp
	timestamp := time.Now().Format("20060102150405")
	taggedImageName := fmt.Sprintf("%s:%s", imageName, timestamp)

	pod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
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