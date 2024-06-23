package container

import (
    "context"
    "fmt"
    "log"
    "time"

    batchv1 "k8s.io/api/batch/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
)

// CreateBuildJob creates a Kubernetes Job to run a Kaniko build.
func CreateBuildJob(clientset *kubernetes.Clientset, name, namespace, configMapName, imageName, dockerSecretName, repoDir, gitRepo, branch, sshKey, pvcName string) (string, string, error) {

    // Generate a unique job name using the current timestamp
    timestamp := time.Now().Format("20060102150405")
    jobName := fmt.Sprintf("%s-docker-build-job-%s", name, timestamp)

    // Generate a unique tag using the current timestamp
    taggedImageName := fmt.Sprintf("%s:%s", imageName, timestamp)

    ttl := int32(1800) // TTL in seconds (30 mins)

    job := &batchv1.Job{
        ObjectMeta: metav1.ObjectMeta{
            Name: jobName,
            Labels: map[string]string{
                "appbuild": name,
            },
        },
        Spec: batchv1.JobSpec{
            Template: corev1.PodTemplateSpec{
                Spec: corev1.PodSpec{
                    InitContainers: []corev1.Container{
                        {
                            Name:  "git-clone",
                            Image: "docker.io/alustan/git-clone:0.4.0",
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
                                "--dockerfile=/workspace/tmp/" + name + "/Dockerfile",
                                "--destination=" + taggedImageName,
                                "--context=/workspace/tmp/" + name,
                            },
                            Env: []corev1.EnvVar{
                                {
                                    Name:  "DOCKER_CONFIG",
                                    Value: "/root/.docker",
                                },
                            },
                            VolumeMounts: []corev1.VolumeMount{
                                {
                                    Name:      "workspace",
                                    MountPath: "/workspace",
                                },
                                {
                                    Name:      "docker-credentials",
                                    MountPath: "/root/.docker",
                                },
                                {
                                    Name:      "dockerfile-config",
                                    MountPath: "/workspace/tmp/" + name + "/Dockerfile",
                                    SubPath:   "Dockerfile",
                                },
                            },
                        },
                    },
                    RestartPolicy: corev1.RestartPolicyNever,
                    Volumes: []corev1.Volume{
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
                    },
                },
            },
            TTLSecondsAfterFinished: &ttl,
        },
    }

    // Create the job
    _, err := clientset.BatchV1().Jobs(namespace).Create(context.Background(), job, metav1.CreateOptions{})
    if err != nil {
        log.Printf("Failed to create Job: %v", err)
        return "", "", err
    }

    log.Printf("Created Job: %s", jobName)
    log.Printf("Image will be pushed with tag: %s", taggedImageName)
    return taggedImageName, jobName, nil
}
