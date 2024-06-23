package container

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "strings"
    "time"

    batchv1 "k8s.io/api/batch/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
)

// CreateRunJob creates a Kubernetes Job that runs a script with specified environment variables and image.
func CreateRunJob(clientset *kubernetes.Clientset, name, namespace, scriptName string, envVars map[string]string, taggedImageName, imagePullSecretName string) (string, error) {
    // Generate a unique job name using the current timestamp
    timestamp := time.Now().Format("20060102150405")
    jobName := fmt.Sprintf("%s-docker-run-job-%s", name, timestamp)

    log.Printf("Creating Job in namespace: %s with image: %s", namespace, taggedImageName)

    env := []corev1.EnvVar{}
    for key, value := range envVars {
        env = append(env, corev1.EnvVar{
            Name:  key,
            Value: value,
        })
        log.Printf("Setting environment variable %s=%s", key, value)
    }

    // Add the script name as an environment variable
    env = append(env, corev1.EnvVar{
        Name:  "SCRIPT",
        Value: scriptName,
    })

    ttl := int32(1800) // TTL in seconds (30 mins)

    job := &batchv1.Job{
        ObjectMeta: metav1.ObjectMeta{
            Name: jobName,
            Labels: map[string]string{
                "apprun": name,
            },
        },
        Spec: batchv1.JobSpec{
            Template: corev1.PodTemplateSpec{
                Spec: corev1.PodSpec{
                    Containers: []corev1.Container{
                        {
                            Name:            "terraform",
                            Image:           taggedImageName,
                            ImagePullPolicy: corev1.PullAlways,
                            Env:             env,
                        },
                    },
                    RestartPolicy: corev1.RestartPolicyNever,
                    ImagePullSecrets: []corev1.LocalObjectReference{
                        {
                            Name: imagePullSecretName,
                        },
                    },
                },
            },
            TTLSecondsAfterFinished: &ttl,
        },
    }

    log.Println("Creating the Job...")
    _, err := clientset.BatchV1().Jobs(namespace).Create(context.Background(), job, metav1.CreateOptions{})
    if err != nil {
        log.Printf("Failed to create Job: %v", err)
        return "", err
    }

    log.Println("Job created successfully.")
    return jobName, nil
}

// WaitForJobCompletion waits for the job to complete and retrieves the output.
func WaitForJobCompletion(clientset *kubernetes.Clientset, namespace, jobName string) (map[string]interface{}, error) {
    for {
        job, err := clientset.BatchV1().Jobs(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
        if err != nil {
            return nil, err
        }
        if job.Status.Succeeded > 0 || job.Status.Failed > 0 {
            break
        }
        time.Sleep(2 * time.Minute)
    }

    pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
        LabelSelector: fmt.Sprintf("job-name=%s", jobName),
    })
    if err != nil {
        return nil, err
    }

    if len(pods.Items) == 0 {
        return nil, fmt.Errorf("no pods found for job %s", jobName)
    }

    podName := pods.Items[0].Name

    req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{})
    logs, err := req.Stream(context.Background())
    if err != nil {
        return nil, err
    }
    defer logs.Close()

    logsBytes, err := io.ReadAll(logs)
    if err != nil {
        return nil, err
    }

    logsString := string(logsBytes)

    // Assuming the JSON output is printed as the last line of the logs
    lines := strings.Split(logsString, "\n")
    lastLine := lines[len(lines)-1]
    if lastLine == "" && len(lines) > 1 {
        lastLine = lines[len(lines)-2]
    }

    var output map[string]interface{}
    err = json.Unmarshal([]byte(lastLine), &output)
    if err != nil {
        return nil, fmt.Errorf("failed to parse output: %v", err)
    }

    return output, nil
}
