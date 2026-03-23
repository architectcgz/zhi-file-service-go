package bootstrap_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	yaml "go.yaml.in/yaml/v2"
)

func TestUploadHelmValuesKeepAuthSecretsOutOfStaticEnv(t *testing.T) {
	values := loadHelmValues(t, "deployments/helm/upload-service/values.yaml")

	assertEnvKeyAbsent(t, values.Env, "UPLOAD_AUTH_JWKS")
	assertEnvKeyAbsent(t, values.Env, "UPLOAD_AUTH_ALLOWED_ISSUERS")
	assertSecretRefsContain(t, values.SecretRefs, "zhi-file-service-shared-secrets")
}

func TestAccessHelmValuesKeepAuthSecretsOutOfStaticEnv(t *testing.T) {
	values := loadHelmValues(t, "deployments/helm/access-service/values.yaml")

	assertEnvKeyAbsent(t, values.Env, "ACCESS_AUTH_JWKS")
	assertEnvKeyAbsent(t, values.Env, "ACCESS_AUTH_ALLOWED_ISSUERS")
	assertSecretRefsContain(t, values.SecretRefs, "zhi-file-service-shared-secrets")
}

func TestUploadKustomizeBaseKeepsAuthSecretsOutOfStaticEnv(t *testing.T) {
	deployment := loadKustomizeDeployment(t, "deployments/kustomize/base/upload-service.yaml")

	assertContainerEnvNameAbsent(t, deployment, "UPLOAD_AUTH_JWKS")
	assertContainerEnvNameAbsent(t, deployment, "UPLOAD_AUTH_ALLOWED_ISSUERS")
	assertContainerSecretRef(t, deployment, "zhi-file-service-shared-secrets")
}

func TestAccessKustomizeBaseKeepsAuthSecretsOutOfStaticEnv(t *testing.T) {
	deployment := loadKustomizeDeployment(t, "deployments/kustomize/base/access-service.yaml")

	assertContainerEnvNameAbsent(t, deployment, "ACCESS_AUTH_JWKS")
	assertContainerEnvNameAbsent(t, deployment, "ACCESS_AUTH_ALLOWED_ISSUERS")
	assertContainerSecretRef(t, deployment, "zhi-file-service-shared-secrets")
}

func TestUploadHelmTemplatePreservesSecretRefsToEnvFrom(t *testing.T) {
	template := loadTextFile(t, "deployments/helm/upload-service/templates/deployment.yaml")

	assertHelmTemplateSecretRefWiring(t, template)
}

func TestAccessHelmTemplatePreservesSecretRefsToEnvFrom(t *testing.T) {
	template := loadTextFile(t, "deployments/helm/access-service/templates/deployment.yaml")

	assertHelmTemplateSecretRefWiring(t, template)
}

type helmValues struct {
	Env        map[string]string `yaml:"env"`
	SecretRefs []string          `yaml:"secretRefs"`
}

type deploymentManifest struct {
	Kind string `yaml:"kind"`
	Spec struct {
		Template struct {
			Spec struct {
				Containers []containerSpec `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

type containerSpec struct {
	Name    string         `yaml:"name"`
	Env     []envVar       `yaml:"env"`
	EnvFrom []envFromEntry `yaml:"envFrom"`
}

type envVar struct {
	Name string `yaml:"name"`
}

type envFromEntry struct {
	SecretRef *secretRef `yaml:"secretRef"`
}

type secretRef struct {
	Name string `yaml:"name"`
}

func loadHelmValues(t *testing.T, relativePath string) helmValues {
	t.Helper()

	var values helmValues
	loadYAML(t, relativePath, &values)
	return values
}

func loadKustomizeDeployment(t *testing.T, relativePath string) deploymentManifest {
	t.Helper()

	repoRoot := findRepoRoot(t)
	content, err := os.ReadFile(filepath.Join(repoRoot, relativePath))
	if err != nil {
		t.Fatalf("read %s: %v", relativePath, err)
	}

	for _, document := range splitYAMLDocuments(string(content)) {
		var manifest deploymentManifest
		if err := yaml.Unmarshal([]byte(document), &manifest); err != nil {
			t.Fatalf("unmarshal %s deployment doc: %v", relativePath, err)
		}
		if manifest.Kind == "Deployment" {
			if len(manifest.Spec.Template.Spec.Containers) == 0 {
				t.Fatalf("%s deployment has no containers", relativePath)
			}
			return manifest
		}
	}

	t.Fatalf("%s does not contain a Deployment document", relativePath)
	return deploymentManifest{}
}

func loadYAML(t *testing.T, relativePath string, target any) {
	t.Helper()

	content := mustReadRepoFile(t, relativePath)
	if err := yaml.Unmarshal(content, target); err != nil {
		t.Fatalf("unmarshal %s: %v", relativePath, err)
	}
}

func loadTextFile(t *testing.T, relativePath string) string {
	t.Helper()

	return string(mustReadRepoFile(t, relativePath))
}

func mustReadRepoFile(t *testing.T, relativePath string) []byte {
	t.Helper()

	repoRoot := findRepoRoot(t)
	content, err := os.ReadFile(filepath.Join(repoRoot, relativePath))
	if err != nil {
		t.Fatalf("read %s: %v", relativePath, err)
	}
	return content
}

func splitYAMLDocuments(content string) []string {
	parts := strings.Split(content, "\n---")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	return result
}

func assertEnvKeyAbsent(t *testing.T, env map[string]string, key string) {
	t.Helper()

	if _, ok := env[key]; ok {
		t.Fatalf("static env %q must stay unset so deployment secret/envFrom can provide it", key)
	}
}

func assertSecretRefsContain(t *testing.T, secretRefs []string, required string) {
	t.Helper()

	for _, secretRef := range secretRefs {
		if strings.TrimSpace(secretRef) == required {
			return
		}
	}
	t.Fatalf("secretRefs=%v, want to contain %q", secretRefs, required)
}

func assertContainerEnvNameAbsent(t *testing.T, manifest deploymentManifest, key string) {
	t.Helper()

	for _, env := range manifest.Spec.Template.Spec.Containers[0].Env {
		if strings.TrimSpace(env.Name) == key {
			t.Fatalf("container env %q must stay unset so deployment secret/envFrom can provide it", key)
		}
	}
}

func assertContainerSecretRef(t *testing.T, manifest deploymentManifest, required string) {
	t.Helper()

	for _, envFrom := range manifest.Spec.Template.Spec.Containers[0].EnvFrom {
		if envFrom.SecretRef != nil && strings.TrimSpace(envFrom.SecretRef.Name) == required {
			return
		}
	}
	t.Fatalf("deployment envFrom secretRefs do not contain %q", required)
}

func assertHelmTemplateSecretRefWiring(t *testing.T, template string) {
	t.Helper()

	requiredSnippets := []string{
		"{{- if .Values.secretRefs }}",
		"envFrom:",
		"{{- range .Values.secretRefs }}",
		"- secretRef:",
		`name: {{ . | quote }}`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(template, snippet) {
			t.Fatalf("helm deployment template is missing required secretRefs/envFrom wiring snippet %q", snippet)
		}
	}
}
