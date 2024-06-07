package container

import (
    "context"
    "log"
    "time"

    batchv1 "k8s.io/api/batch/v1"
    corev1 "k8s.io/api/core/v1"
    apierrors "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/types"
    "k8s.io/client-go/kubernetes"
)

// Retry parameters
const (
    retryInterval = 2 * time.Second
    maxRetries    = 10
)

// removeFinalizers removes all finalizers from the job
func removeFinalizers(clientset *kubernetes.Clientset, namespace, jobName string) error {
    patch := []byte(`{"metadata":{"finalizers":[]}}`)
    _, err := clientset.BatchV1().Jobs(namespace).Patch(context.Background(), jobName, types.MergePatchType, patch, metav1.PatchOptions{})
    if err != nil {
        log.Printf("Failed to remove finalizers from Job: %v", err)
    }
    return err
}

// CreateBuildJob creates a Kubernetes Job to run a Kaniko build
func CreateBuildJob(clientset *kubernetes.Clientset, namespace, configMapName, imageName, dockerSecretName string) error {
    jobName := "docker-build-job"

    // Attempt to get the existing job
    job, err := clientset.BatchV1().Jobs(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
    if err == nil {
        // Job exists, attempt to remove finalizers
        if len(job.ObjectMeta.Finalizers) > 0 {
            log.Printf("Removing finalizers from Job: %s", jobName)
            err := removeFinalizers(clientset, namespace, jobName)
            if err != nil {
                return err
            }
        }

        // Delete the job
        deletePolicy := metav1.DeletePropagationForeground
        err = clientset.BatchV1().Jobs(namespace).Delete(context.Background(), jobName, metav1.DeleteOptions{
            PropagationPolicy: &deletePolicy,
        })
        if err != nil {
            log.Printf("Failed to delete existing Job: %v", err)
            return err
        }
        log.Printf("Deleted existing Job: %s", jobName)
    } else if !apierrors.IsNotFound(err) {
        log.Printf("Error checking for existing Job: %v", err)
        return err
    } else {
        log.Printf("No existing Job to delete: %s", jobName)
    }

    // Retry loop to wait for the job to be fully deleted
    for i := 0; i < maxRetries; i++ {
        _, err := clientset.BatchV1().Jobs(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
        if apierrors.IsNotFound(err) {
            // Job does not exist, safe to create
            break
        } else if err != nil {
            // If error is not NotFound, something went wrong
            log.Printf("Error checking if job exists: %v", err)
            return err
        }
        log.Printf("Job %s is still being deleted, retrying...", jobName)
        time.Sleep(time.Duration(i+1) * retryInterval)
    }

    job = &batchv1.Job{
        ObjectMeta: metav1.ObjectMeta{
            Name: jobName,
        },
        Spec: batchv1.JobSpec{
            Template: corev1.PodTemplateSpec{
                Spec: corev1.PodSpec{
                    Containers: []corev1.Container{
                        {
                            Name:  "kaniko",
                            Image: "gcr.io/kaniko-project/executor:v1.23.0",
                            Args: []string{
                                "--dockerfile=/config/Dockerfile",
                                "--destination=" + imageName,
                                "--context=/workspace/",
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
                                    MountPath: "/kaniko/.docker",
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
                    },
                    ImagePullSecrets: []corev1.LocalObjectReference{
                        {
                            Name: dockerSecretName,
                        },
                    },
                },
            },
        },
    }

    // Create the job
    _, err = clientset.BatchV1().Jobs(namespace).Create(context.Background(), job, metav1.CreateOptions{})
    if err != nil {
        log.Printf("Failed to create Job: %v", err)
        return err
    }

    log.Printf("Created Job: %s", jobName)
    return nil
}
