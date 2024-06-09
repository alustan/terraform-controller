package container

import (
	"context"
	"fmt"
	"log"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// RemoveFinalizersFromPod removes all finalizers from the specified Pod.
func RemoveFinalizersFromPod(clientset *kubernetes.Clientset, namespace, podName string) error {
	patch := []byte(`{"metadata":{"finalizers":[]}}`)
	_, err := clientset.CoreV1().Pods(namespace).Patch(context.Background(), podName, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		log.Printf("Failed to remove finalizers from Pod: %v", err)
		return err
	}
	log.Printf("Finalizers removed from Pod: %s", podName)
	return nil
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

// deleteRunPodWithRetry attempts to delete a Pod with retry logic
func deleteRunPodWithRetry(clientset *kubernetes.Clientset, namespace, podName string) error {
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

// DeletePodIfExists deletes the Pod if it already exists, including removing finalizers if present.
func DeletePodIfExists(clientset *kubernetes.Clientset, namespace, podName string) error {
	// Attempt to get the existing Pod
	pod, err := clientset.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Printf("Pod %s not found in namespace %s", podName, namespace)
			return nil
		}
		log.Printf("Failed to get Pod %s: %v", podName, err)
		return err
	}

	// If the Pod has finalizers, remove them
	if len(pod.ObjectMeta.Finalizers) > 0 {
		log.Printf("Removing finalizers from Pod: %s", podName)
		err := RemoveFinalizersFromPod(clientset, namespace, podName)
		if err != nil {
			return err
		}
	}

	// Delete the Pod with retry logic
	err = deleteRunPodWithRetry(clientset, namespace, podName)
	if err != nil {
		return err
	}

	// Wait for pod deletion to complete
	return WaitForPodDeletion(clientset, namespace, podName)
}

// CreateRunPod creates a Kubernetes Pod that runs a script with specified environment variables and image.
func CreateRunPod(clientset *kubernetes.Clientset, name, namespace string, envVars map[string]string, scriptContent, taggedImageName, pvcName, imagePullSecretName string) error {
	err := EnsurePVC(clientset, namespace, pvcName)
	if err != nil {
		log.Printf("Failed to ensure PVC: %v", err)
		return err
	}

	podName := fmt.Sprintf("%s-docker-run-pod", name)

	// Attempt to delete the existing pod if it exists
	err = DeletePodIfExists(clientset, namespace, podName)
	if err != nil {
		return err
	}

	log.Printf("Creating Pod in namespace: %s with image: %s", namespace, taggedImageName)

	env := []v1.EnvVar{}
	for key, value := range envVars {
		env = append(env, v1.EnvVar{
			Name:  key,
			Value: value,
		})
		log.Printf("Setting environment variable %s=%s", key, value)
	}

	// Write the script content to a file inside the container
	scriptPath := "/workspace/script.sh"
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
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
						fmt.Sprintf("echo '%s' > %s && chmod +x %s && %s", scriptContent, scriptPath, scriptPath, scriptPath),
					},
					Env: env,
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "terraform-pv",
							MountPath: "/workspace",
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
			Volumes: []v1.Volume{
				{
					Name: "terraform-pv",
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
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
		return err
	}

	log.Println("Pod created successfully.")
	return nil
}
