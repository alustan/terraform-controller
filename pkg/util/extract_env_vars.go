package util


func ExtractEnvVars(variables, backend map[string]string) map[string]string {
	envVars := make(map[string]string)
	for key, value := range variables {
		envVars[key] = value
	}
	for key, value := range backend {
		envVars[key] = value
	}
	return envVars
}

