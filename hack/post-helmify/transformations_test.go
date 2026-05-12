package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Helper test utilities ---

func setupTestChart(t *testing.T) (Config, string) {
	t.Helper()
	dir := t.TempDir()
	chartDir := filepath.Join(dir, "chart")
	templatesDir := filepath.Join(chartDir, "templates")
	srcDir := filepath.Join(dir, "config", "tmp")
	os.MkdirAll(templatesDir, 0755)
	os.MkdirAll(srcDir, 0755)
	return Config{ChartPath: chartDir, TemplateSrcDir: srcDir}, dir
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// --- String helper tests ---

func TestInsertAfterLine(t *testing.T) {
	input := "line1\nline2\nline3"
	result := insertAfterLine(input, "line2", "inserted")
	expected := "line1\nline2\ninserted\nline3"
	if result != expected {
		t.Errorf("got:\n%s\nwant:\n%s", result, expected)
	}
}

func TestInsertAfterLine_NotFound(t *testing.T) {
	input := "line1\nline2\nline3"
	result := insertAfterLine(input, "nothere", "inserted")
	if result != input {
		t.Errorf("expected no change, got:\n%s", result)
	}
}

func TestInsertBeforeLine(t *testing.T) {
	input := "line1\nline2\nline3"
	result := insertBeforeLine(input, "line2", "inserted")
	expected := "line1\ninserted\nline2\nline3"
	if result != expected {
		t.Errorf("got:\n%s\nwant:\n%s", result, expected)
	}
}

func TestRemoveLineContaining(t *testing.T) {
	input := "keep1\nremove this\nkeep2\nalso remove this\nkeep3"
	result := removeLineContaining(input, "remove")
	expected := "keep1\nkeep2\nkeep3"
	if result != expected {
		t.Errorf("got:\n%s\nwant:\n%s", result, expected)
	}
}

func TestReplaceEnvValue(t *testing.T) {
	input := `        - name: MY_VAR
          value: "old-value"
        - name: OTHER`
	result := replaceEnvValue(input, "MY_VAR", `{{ .new }}`)
	if !strings.Contains(result, `value: {{ .new }}`) {
		t.Errorf("replacement failed, got:\n%s", result)
	}
	if strings.Contains(result, "old-value") {
		t.Error("old value still present")
	}
}

func TestHasKeyUnderSection(t *testing.T) {
	input := "serviceAccount:\n  labels: {}\n  name: foo"
	if !hasKeyUnderSection(input, "serviceAccount:", "labels:") {
		t.Error("expected to find labels under serviceAccount")
	}
	if hasKeyUnderSection(input, "serviceAccount:", "missing:") {
		t.Error("should not find missing key")
	}
}

// --- Deployment patch tests ---

func TestPatchDeploymentPreStop(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := `        securityContext:
          allowPrivilegeEscalation: false
        volumeMounts:
        - mountPath: /tmp/certs
          name: webhook-certs`
	writeTestFile(t, cfg.deployment(), content)

	if err := patchDeploymentPreStop(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.deployment())
	if !strings.Contains(result, "preStop") {
		t.Error("preStop hook not inserted")
	}
	if !strings.Contains(result, `["sleep", "5"]`) {
		t.Error("sleep command not present")
	}
	// volumeMounts should still be present after the hook
	if !strings.Contains(result, "volumeMounts:") {
		t.Error("volumeMounts removed")
	}
}

func TestPatchDeploymentPreStop_Idempotent(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := `        lifecycle:
          preStop:
            exec:
              command: ["sleep", "5"]
        volumeMounts:
        - mountPath: /tmp/certs`
	writeTestFile(t, cfg.deployment(), content)

	if err := patchDeploymentPreStop(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.deployment())
	if result != content {
		t.Error("expected no change on already-patched content")
	}
}

func TestPatchDeploymentPodLabels(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := `spec:
  template:
    metadata:
      labels:
        control-plane: controller-manager
        app.kubernetes.io/name: git-hubby
    spec:
      containers: []`
	writeTestFile(t, cfg.deployment(), content)

	if err := patchDeploymentPodLabels(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.deployment())
	if !strings.Contains(result, ".Values.controllerManager.podLabels") {
		t.Error("podLabels not inserted")
	}
	// Should not be in the first line
	lines := strings.Split(result, "\n")
	for i, l := range lines {
		if strings.Contains(l, "control-plane: controller-manager") {
			if i+1 >= len(lines) || !strings.Contains(lines[i+1], "podLabels") {
				t.Error("podLabels not inserted after control-plane label")
			}
			break
		}
	}
}

func TestPatchDeploymentPodLabels_Idempotent(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := `        control-plane: controller-manager
        {{- with .Values.controllerManager.podLabels }}
        {{ toYaml . | nindent 8 }}
        {{- end }}`
	writeTestFile(t, cfg.deployment(), content)

	if err := patchDeploymentPodLabels(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.deployment())
	if result != content {
		t.Error("expected no change on already-patched content")
	}
}

func TestPatchDeploymentStrategy(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := `spec:
  replicas: 2
  strategy:
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
    type: RollingUpdate
  selector:
    matchLabels:
      control-plane: controller-manager`
	writeTestFile(t, cfg.deployment(), content)

	if err := patchDeploymentStrategy(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.deployment())
	if !strings.Contains(result, "with .Values.controllerManager.strategy") {
		t.Error("strategy not replaced with values-driven block")
	}
	if !strings.Contains(result, "toYaml . | nindent 4") {
		t.Error("toYaml helper not present")
	}
	if strings.Contains(result, "rollingUpdate:") {
		t.Error("hardcoded rollingUpdate still present")
	}
	if strings.Contains(result, "maxSurge:") {
		t.Error("hardcoded maxSurge still present")
	}
	// selector: line should still be present
	if !strings.Contains(result, "selector:") {
		t.Error("selector line was removed")
	}
}

func TestPatchDeploymentStrategy_Idempotent(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := `spec:
  replicas: 2
  {{- with .Values.controllerManager.strategy }}
  strategy: {{- toYaml . | nindent 4 }}
  {{- end }}
  selector:
    matchLabels:
      control-plane: controller-manager`
	writeTestFile(t, cfg.deployment(), content)

	if err := patchDeploymentStrategy(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.deployment())
	if result != content {
		t.Error("expected no change on already-patched content")
	}
}

func TestPatchDeploymentStrategy_MissingFile(t *testing.T) {
	cfg, _ := setupTestChart(t)
	// deployment file does not exist — should be a no-op
	if err := patchDeploymentStrategy(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReplaceConfigMapRefName(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := `        envFrom:
        - configMapRef:
            name: controller-manager-envs`
	writeTestFile(t, cfg.deployment(), content)

	if err := replaceConfigMapRefName(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.deployment())
	if !strings.Contains(result, `{{ include "chart.fullname" . }}-envs`) {
		t.Error("configMapRef name not replaced")
	}
}

func TestReplaceWatchNamespaceValue(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := `        - name: WATCH_NAMESPACE
          value: "git-hubby-system"`
	writeTestFile(t, cfg.deployment(), content)

	if err := replaceWatchNamespaceValue(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.deployment())
	if !strings.Contains(result, "chart.watchNamespace") {
		t.Error("WATCH_NAMESPACE not replaced with helper")
	}
}

func TestReplaceWatchNamespaceValue_Idempotent(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := `        - name: WATCH_NAMESPACE
          value: {{ include "chart.watchNamespace" . | quote }}`
	writeTestFile(t, cfg.deployment(), content)

	if err := replaceWatchNamespaceValue(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.deployment())
	if result != content {
		t.Error("expected no change")
	}
}

func TestTemplateAppCredentialsEnv(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := `        - name: APP_CREDENTIALS_SECRET_NAMESPACE
          value: "git-hubby-system"`
	writeTestFile(t, cfg.deployment(), content)

	if err := templateAppCredentialsEnv(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.deployment())
	if !strings.Contains(result, "appCredentialsSecretNamespace") {
		t.Error("APP_CREDENTIALS_SECRET_NAMESPACE not templated")
	}
	if strings.Contains(result, `"git-hubby-system"`) {
		t.Error("hardcoded value still present")
	}
}

// --- ServiceAccount tests ---

func TestPatchServiceAccountSecrets(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := `apiVersion: v1
kind: ServiceAccount
metadata:
  name: test
automountServiceAccountToken: true`
	writeTestFile(t, cfg.serviceAccount(), content)

	if err := patchServiceAccountSecrets(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.serviceAccount())
	if !strings.Contains(result, "serviceAccount.secrets") {
		t.Error("secrets not added")
	}
}

func TestPatchServiceAccountLabels(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := `metadata:
  labels:
  {{- include "chart.labels" . | nindent 4 }}
  name: test`
	writeTestFile(t, cfg.serviceAccount(), content)

	if err := patchServiceAccountLabels(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.serviceAccount())
	if !strings.Contains(result, "serviceAccount.labels") {
		t.Error("labels not added")
	}
}

// --- RBAC tests ---

func TestCopyManagerRBACTemplate(t *testing.T) {
	cfg, _ := setupTestChart(t)
	srcContent := "# multi-namespace template"
	dstContent := "# helmify generated"
	writeTestFile(t, cfg.managerRBACSrc(), srcContent)
	writeTestFile(t, cfg.managerRBAC(), dstContent)

	if err := copyManagerRBACTemplate(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.managerRBAC())
	if result != srcContent {
		t.Error("manager-rbac not copied from source")
	}
}

func TestPatchAppCredentialsRBAC(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: test-role
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: test-rolebinding`
	writeTestFile(t, cfg.appCredsRBAC(), content)

	if err := patchAppCredentialsRBAC(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.appCredsRBAC())
	if count := strings.Count(result, "appCredentialsSecretNamespace"); count != 2 {
		t.Errorf("expected 2 namespace insertions, got %d", count)
	}
}

// --- Webhook tests ---

func TestTemplateWebhookNamespaceSelector(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := `webhooks:
- name: test
  namespaceSelector:
    matchLabels:
      kubernetes.io/metadata.name: git-hubby-system
  rules: []`
	writeTestFile(t, cfg.webhook(), content)

	if err := templateWebhookNamespaceSelector(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.webhook())
	if !strings.Contains(result, "chart.webhookNamespaceSelector") {
		t.Error("namespaceSelector not replaced with helper")
	}
	if strings.Contains(result, "matchLabels") {
		t.Error("matchLabels still present")
	}
}

func TestTemplateWebhookNamespaceSelector_Idempotent(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := `  {{- include "chart.webhookNamespaceSelector" . | nindent 2 }}`
	writeTestFile(t, cfg.webhook(), content)

	if err := templateWebhookNamespaceSelector(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.webhook())
	if result != content {
		t.Error("expected no change")
	}
}

// --- PDB tests ---

func TestCopyPDBTemplate(t *testing.T) {
	cfg, _ := setupTestChart(t)
	srcContent := `{{- with .Values.controllerManager.podDisruptionBudget }}
{{- if .enabled }}
apiVersion: policy/v1
kind: PodDisruptionBudget
{{- end }}
{{- end }}
`
	dstContent := "# helmify generated"
	writeTestFile(t, cfg.pdbSrc(), srcContent)
	writeTestFile(t, cfg.pdb(), dstContent)

	if err := copyPDBTemplate(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.pdb())
	if result != srcContent {
		t.Error("PDB not copied from source")
	}
}

func TestCopyPDBTemplate_MissingSrc(t *testing.T) {
	cfg, _ := setupTestChart(t)
	writeTestFile(t, cfg.pdb(), "# helmify generated")
	// src does not exist — should be a no-op
	if err := copyPDBTemplate(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCopyPDBTemplate_MissingDst(t *testing.T) {
	cfg, _ := setupTestChart(t)
	srcContent := "# source template"
	writeTestFile(t, cfg.pdbSrc(), srcContent)
	// dst does not exist — should be created from source
	if err := copyPDBTemplate(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := readTestFile(t, cfg.pdb())
	if result != srcContent {
		t.Errorf("expected destination to be created with source content, got:\n%s", result)
	}
}

func TestAddPDBValues(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := `controllerManager:
  replicas: 2
  strategy:
    type: RollingUpdate`
	writeTestFile(t, cfg.values(), content)

	if err := addPDBValues(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.values())
	if !strings.Contains(result, "podDisruptionBudget:") {
		t.Error("podDisruptionBudget not added")
	}
	if !strings.Contains(result, "enabled: true") {
		t.Error("enabled default not set")
	}
	if !strings.Contains(result, "minAvailable: 1") {
		t.Error("minAvailable default not set")
	}
}

func TestAddPDBValues_Idempotent(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := `controllerManager:
  replicas: 2
  podDisruptionBudget:
    enabled: true
    minAvailable: 1`
	writeTestFile(t, cfg.values(), content)

	if err := addPDBValues(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.values())
	if strings.Count(result, "podDisruptionBudget") != 1 {
		t.Error("podDisruptionBudget duplicated")
	}
}

func TestAddTopologySpreadValues(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := `controllerManager:
  tolerations: []
  topologySpreadConstraints: []`
	writeTestFile(t, cfg.values(), content)

	if err := addTopologySpreadValues(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.values())
	if !strings.Contains(result, "topologyKey: kubernetes.io/hostname") {
		t.Error("topologySpreadConstraints not added")
	}
	if !strings.Contains(result, "maxSkew: 1") {
		t.Error("maxSkew not set")
	}
	if !strings.Contains(result, "ScheduleAnyway") {
		t.Error("whenUnsatisfiable not set")
	}
	if strings.Contains(result, "topologySpreadConstraints: []") {
		t.Error("empty placeholder still present")
	}
}

func TestAddTopologySpreadValues_Idempotent(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := `controllerManager:
  topologySpreadConstraints:
    - maxSkew: 1
      topologyKey: kubernetes.io/hostname
      whenUnsatisfiable: ScheduleAnyway`
	writeTestFile(t, cfg.values(), content)

	if err := addTopologySpreadValues(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.values())
	if result != content {
		t.Error("expected no change on already-configured content")
	}
}

func TestPatchDeploymentTopologySpread(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := `spec:
  template:
    spec:
      tolerations: []
      topologySpreadConstraints: {{- toYaml .Values.controllerManager.topologySpreadConstraints
        | nindent 8 }}
      volumes: []`
	writeTestFile(t, cfg.deployment(), content)

	if err := patchDeploymentTopologySpread(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.deployment())
	// Should use omit to preserve all fields
	if !strings.Contains(result, `omit . "labelSelector"`) {
		t.Error("omit not used for full field preservation")
	}
	// Should inject default labelSelector with chart.selectorLabels
	if !strings.Contains(result, "chart.selectorLabels") {
		t.Error("chart.selectorLabels not injected")
	}
	// Should support custom labelSelector pass-through
	if !strings.Contains(result, "if .labelSelector") {
		t.Error("custom labelSelector support not present")
	}
	// Should use with block so empty constraints produce no output
	if !strings.Contains(result, "with .Values.controllerManager.topologySpreadConstraints") {
		t.Error("with block not present")
	}
	// volumes: should still be present after the block
	if !strings.Contains(result, "volumes:") {
		t.Error("volumes line was removed")
	}
}

func TestPatchDeploymentTopologySpread_Idempotent(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := `      {{- with .Values.controllerManager.topologySpreadConstraints }}
      topologySpreadConstraints:
        {{- range . }}
        - {{- toYaml (omit . "labelSelector") | nindent 10 }}
          {{- if .labelSelector }}
          labelSelector: {{- toYaml .labelSelector | nindent 12 }}
          {{- else }}
          labelSelector:
            matchLabels:
              control-plane: controller-manager
              {{- include "chart.selectorLabels" $ | nindent 14 }}
          {{- end }}
        {{- end }}
      {{- end }}`
	writeTestFile(t, cfg.deployment(), content)

	if err := patchDeploymentTopologySpread(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.deployment())
	if result != content {
		t.Errorf("expected no change on already-patched content, got:\n%s", result)
	}
}

func TestPatchDeploymentTopologySpread_MissingFile(t *testing.T) {
	cfg, _ := setupTestChart(t)
	if err := patchDeploymentTopologySpread(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPatchDeploymentTopologySpread_SelectorLabelsElsewhere(t *testing.T) {
	cfg, _ := setupTestChart(t)
	// Simulates a real deployment where chart.selectorLabels already appears
	// in selector.matchLabels — the patch must still replace the raw toYaml block.
	content := `spec:
  selector:
    matchLabels:
      {{- include "chart.selectorLabels" . | nindent 6 }}
  template:
    spec:
      topologySpreadConstraints: {{- toYaml .Values.controllerManager.topologySpreadConstraints
        | nindent 8 }}
      volumes: []`
	writeTestFile(t, cfg.deployment(), content)

	if err := patchDeploymentTopologySpread(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.deployment())
	if !strings.Contains(result, `omit . "labelSelector"`) {
		t.Error("patch was not applied despite raw toYaml block being present")
	}
	if strings.Contains(result, "toYaml .Values.controllerManager.topologySpreadConstraints") {
		t.Error("raw toYaml block was not replaced")
	}
}

// --- Values.yaml tests ---

func TestPatchValuesNamespaces(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := `controllerManager:
  manager:
    env:
      watchNamespace: git-hubby-system
      appCredentialsSecretNamespace: git-hubby-system
    image:
      repository: test`
	writeTestFile(t, cfg.values(), content)

	if err := patchValuesNamespaces(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.values())
	if strings.Contains(result, "watchNamespace") {
		t.Error("watchNamespace not removed")
	}
	if strings.Contains(result, "appCredentialsSecretNamespace") {
		t.Error("appCredentialsSecretNamespace not removed")
	}
	if !strings.Contains(result, "watchedNamespaces") {
		t.Error("watchedNamespaces not added")
	}
	if !strings.Contains(result, "github-configuration") {
		t.Error("default namespace not set")
	}
	// Empty env: line should be removed
	for line := range strings.SplitSeq(result, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "env:" {
			t.Error("empty env: key not removed")
		}
	}
}

func TestPatchValuesNamespaces_Idempotent(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := `controllerManager:
  watchedNamespaces:
    - github-configuration
  manager:
    image:
      repository: test`
	writeTestFile(t, cfg.values(), content)

	if err := patchValuesNamespaces(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.values())
	if strings.Count(result, "watchedNamespaces") != 1 {
		t.Error("watchedNamespaces duplicated")
	}
}

func TestPatchValuesDefaults(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := `controllerManager:
  manager:
    image: test
serviceAccount:
  name: test`
	writeTestFile(t, cfg.values(), content)

	if err := patchValuesDefaults(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.values())
	if !strings.Contains(result, "podLabels") {
		t.Error("podLabels not added")
	}
	if !strings.Contains(result, "secrets") {
		t.Error("secrets not added")
	}
	if !strings.Contains(result, "labels:") {
		t.Error("labels not added")
	}
}

func TestAddServingCertValues(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := "controllerManager:\n  manager: {}\n"
	writeTestFile(t, cfg.values(), content)

	if err := addServingCertValues(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.values())
	if !strings.Contains(result, "servingCert:") {
		t.Error("servingCert not added")
	}
	if !strings.Contains(result, "selfsigned-issuer") {
		t.Error("issuerRef not added")
	}
}

func TestAddServingCertValues_Idempotent(t *testing.T) {
	cfg, _ := setupTestChart(t)
	content := "controllerManager: {}\nservingCert:\n  issuerRef:\n    kind: Issuer\n"
	writeTestFile(t, cfg.values(), content)

	if err := addServingCertValues(cfg); err != nil {
		t.Fatal(err)
	}

	result := readTestFile(t, cfg.values())
	if strings.Count(result, "servingCert") != 1 {
		t.Error("servingCert duplicated")
	}
}

// --- Integration test ---

func TestFullRun(t *testing.T) {
	cfg, _ := setupTestChart(t)

	// Create minimal chart files
	writeTestFile(t, cfg.deployment(), `spec:
  template:
    metadata:
      labels:
        control-plane: controller-manager
    spec:
      containers:
      - name: manager
        env:
        - name: WATCH_NAMESPACE
          value: "git-hubby-system"
        - name: APP_CREDENTIALS_SECRET_NAMESPACE
          value: "git-hubby-system"
        envFrom:
        - configMapRef:
            name: controller-manager-envs`)

	writeTestFile(t, cfg.serviceAccount(), `apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
  {{- include "chart.labels" . | nindent 4 }}
  name: test
automountServiceAccountToken: true`)

	writeTestFile(t, cfg.managerRBAC(), "# helmify generated")
	writeTestFile(t, cfg.managerRBACSrc(), "# multi-namespace template")

	writeTestFile(t, cfg.appCredsRBAC(), `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: test`)

	writeTestFile(t, cfg.webhook(), `webhooks:
- name: test
  namespaceSelector:
    matchLabels:
      kubernetes.io/metadata.name: git-hubby-system
  rules: []`)

	writeTestFile(t, cfg.values(), `controllerManager:
  replicas: 2
  manager:
    env:
      watchNamespace: git-hubby-system
      appCredentialsSecretNamespace: git-hubby-system
    image:
      repository: test
serviceAccount:
  name: test`)

	writeTestFile(t, cfg.servingCertSrc(), "# serving cert template")
	writeTestFile(t, cfg.ChartPath+"/templates/serving-cert.yaml", "# old")

	pdbSrcContent := `{{- with .Values.controllerManager.podDisruptionBudget }}
{{- if .enabled }}
apiVersion: policy/v1
kind: PodDisruptionBudget
{{- end }}
{{- end }}
`
	writeTestFile(t, cfg.pdbSrc(), pdbSrcContent)
	writeTestFile(t, cfg.pdb(), "# helmify generated PDB")

	// Run the full orchestration — this tests step ordering, error propagation, and Config injection.
	if err := run(cfg); err != nil {
		t.Fatalf("run() failed: %v", err)
	}

	// Verify key outcomes
	deployment := readTestFile(t, cfg.deployment())
	if !strings.Contains(deployment, "chart.watchNamespace") {
		t.Error("WATCH_NAMESPACE not templated")
	}
	if !strings.Contains(deployment, "appCredentialsSecretNamespace") {
		t.Error("APP_CREDENTIALS not templated")
	}

	values := readTestFile(t, cfg.values())
	if !strings.Contains(values, "watchedNamespaces") {
		t.Error("watchedNamespaces not in values")
	}
	if strings.Contains(values, "watchNamespace") {
		t.Error("watchNamespace still in values")
	}
	if !strings.Contains(values, "podDisruptionBudget") {
		t.Error("podDisruptionBudget not in values")
	}

	pdb := readTestFile(t, cfg.pdb())
	if pdb != pdbSrcContent {
		t.Error("PDB not copied from source template")
	}
}
