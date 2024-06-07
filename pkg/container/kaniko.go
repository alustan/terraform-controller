package container

import (
	"context"
	"log"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CreateBuildJob creates a Kubernetes Job to run a Kaniko build
func CreateBuildJob(clientset *kubernetes.Clientset, namespace, configMapName, imageName, dockerSecretName string) error {
	jobName := "docker-build-job"

	// Attempt to get the existing job
    _, err := clientset.BatchV1().Jobs(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
    if err == nil {
        // Job exists, attempt to delete it
        deletePolicy := metav1.DeletePropagationForeground
        err := clientset.BatchV1().Jobs(namespace).Delete(context.Background(), jobName, metav1.DeleteOptions{
            PropagationPolicy: &deletePolicy,
        })
        if err != nil {
            log.Printf("Failed to delete existing Job: %v", err)
            return err
        }
        log.Printf("Deleted existing Job: %s", jobName)
    } else {
        log.Printf("No existing Job to delete: %s", jobName)
    }

	job := &batchv1.Job{
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
								"--docker-credential-directory=/kaniko/.docker",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "dockerfile-config",
									MountPath: "/config",
									SubPath:   "Dockerfile",
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
								},
							},
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
