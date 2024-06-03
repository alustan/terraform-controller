package container

import (
    "context"
    "fmt"
    "log"

    v1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/api/resource"
    "k8s.io/client-go/kubernetes"
)

// EnsurePVC ensures that the specified Persistent Volume Claim exists.
func EnsurePVC(clientset *kubernetes.Clientset, namespace, pvcName string) error {
    pvc, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), pvcName, metav1.GetOptions{})
    if err == nil && pvc != nil {
        log.Printf("PVC %s already exists in namespace %s", pvcName, namespace)
        return nil
    }

    log.Printf("Creating PVC %s in namespace %s", pvcName, namespace)
    pvc = &v1.PersistentVolumeClaim{
        ObjectMeta: metav1.ObjectMeta{
            Name: pvcName,
        },
        Spec: v1.PersistentVolumeClaimSpec{
            AccessModes: []v1.PersistentVolumeAccessMode{
                v1.ReadWriteOnce,
            },
            Resources: v1.VolumeResourceRequirements{
                Requests: v1.ResourceList{
                    v1.ResourceStorage: resource.MustParse("5Gi"),
                },
            },
        },
    }

    _, err = clientset.CoreV1().PersistentVolumeClaims(namespace).Create(context.Background(), pvc, metav1.CreateOptions{})
    if err != nil {
        log.Printf("Failed to create PVC: %v", err)
        return fmt.Errorf("failed to create PVC: %v", err)
    }

    log.Println("PVC created successfully.")
    return nil
}


// CreateRunPod creates a Kubernetes Pod that runs a script with specified environment variables and image.
func CreateRunPod(clientset *kubernetes.Clientset, namespace string, envVars map[string]string, script, imageName, pvcName string) error {
    err := EnsurePVC(clientset, namespace, pvcName)
    if err != nil {
        return fmt.Errorf("failed to ensure PVC: %v", err)
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
            Name: "docker-run-pod",
        },
        Spec: v1.PodSpec{
            Containers: []v1.Container{
                {
                    Name:  "app",
                    Image: imageName,
                    Command: []string{
                        "/bin/bash",
                        "-c",
                        fmt.Sprintf("chmod +x %s && exec %s", script, script),
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
        },
    }

    log.Println("Creating the Pod...")
    _, err = clientset.CoreV1().Pods(namespace).Create(context.Background(), pod, metav1.CreateOptions{})
    if err != nil {
        log.Printf("Failed to create Pod: %v", err)
        return fmt.Errorf("failed to create Pod: %v", err)
    }

    log.Println("Pod created successfully.")
    return nil
}




