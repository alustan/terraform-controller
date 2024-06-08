package container

import (
    "context"
    "log"
    "fmt"

    v1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    apierrors "k8s.io/apimachinery/pkg/api/errors"
    
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

    // Delete the Pod
    err = clientset.CoreV1().Pods(namespace).Delete(context.Background(), podName, metav1.DeleteOptions{})
    if err != nil {
        log.Printf("Failed to delete Pod %s: %v", podName, err)
        return err
    }
    log.Printf("Deleted existing Pod: %s", podName)
    return nil
}




// CreateRunPod creates a Kubernetes Pod that runs a script with specified environment variables and image.
func CreateRunPod(clientset *kubernetes.Clientset, name, namespace string, envVars map[string]string, script, imageName, pvcName, imagePullSecretName string) error {
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

    log.Printf("Creating Pod in namespace: %s with image: %s", namespace, imageName)

    env := []v1.EnvVar{}
    for key, value := range envVars {
        env = append(env, v1.EnvVar{
            Name:  key,
            Value: value,
        })
        log.Printf("Setting environment variable %s=%s", key, value)
    }

    pod := &v1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name: podName,
        },
        Spec: v1.PodSpec{
            Containers: []v1.Container{
                {
                    Name:  "terraform",
                    Image: imageName,
                    ImagePullPolicy: v1.PullAlways,
                    Command: []string{
                        "/bin/bash",
                        "-c",
                        "chmod +x " + script + " && exec " + script,
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
