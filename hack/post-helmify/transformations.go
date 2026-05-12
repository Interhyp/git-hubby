package main

import (
	"os"
	"regexp"
	"strings"
)

// --- File helpers ---

// readFile reads a file and returns its content. Returns empty string and nil error if file doesn't exist.
func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// writeFile writes content to a file, creating it if necessary.
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

// copyFile copies src to dst. Returns nil if src doesn't exist.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// --- Deployment patches ---

// patchDeploymentPreStop adds a preStop lifecycle hook to the manager container.
// This gives kube-proxy time to remove the pod from Service endpoints before it
// stops serving webhook traffic, preventing "context deadline exceeded" errors
// during rolling updates.
func patchDeploymentPreStop(cfg Config) error {
	content, err := readFile(cfg.deployment())
	if err != nil || content == "" {
		return err
	}
	if strings.Contains(content, "preStop") {
		return nil // already patched
	}
	preStop := `        lifecycle:
          preStop:
            exec:
              command: ["sleep", "5"]`
	content = insertBeforeLine(content, "volumeMounts:", preStop)
	return writeFile(cfg.deployment(), content)
}

// patchDeploymentStrategy replaces helmify's explicit per-field strategy block with a
// conditional toYaml block that works regardless of strategy type or missing keys.
func patchDeploymentStrategy(cfg Config) error {
	content, err := readFile(cfg.deployment())
	if err != nil || content == "" {
		return err
	}
	if strings.Contains(content, "with .Values.controllerManager.strategy") {
		return nil // already patched
	}
	// Match the helmify-generated strategy block (multi-line with rollingUpdate sub-fields).
	// Matches "  strategy:\n" followed by lines indented with 4+ spaces, up to the "  selector:" line.
	re := regexp.MustCompile(`(?m)^  strategy:\n(    .*\n)+`)
	replacement := `  {{- with .Values.controllerManager.strategy }}
  strategy: {{- toYaml . | nindent 4 }}
  {{- end }}
`
	content = re.ReplaceAllString(content, replacement)
	return writeFile(cfg.deployment(), content)
}

// patchDeploymentPodLabels adds podLabels template block to deployment pod template.
func patchDeploymentPodLabels(cfg Config) error {
	content, err := readFile(cfg.deployment())
	if err != nil || content == "" {
		return err
	}
	if strings.Contains(content, ".Values.controllerManager.podLabels") {
		return nil // already patched
	}
	insertion := `        {{- with .Values.controllerManager.podLabels }}
        {{ toYaml . | nindent 8 }}
        {{- end }}`
	content = insertAfterLine(content, "        control-plane: controller-manager", insertion)
	return writeFile(cfg.deployment(), content)
}

// replaceConfigMapRefName replaces hardcoded configmap name with Helm template.
func replaceConfigMapRefName(cfg Config) error {
	content, err := readFile(cfg.deployment())
	if err != nil || content == "" {
		return err
	}
	if !strings.Contains(content, "name: controller-manager-envs") {
		return nil
	}
	content = strings.Replace(content, "name: controller-manager-envs", `name: {{ include "chart.fullname" . }}-envs`, 1)
	return writeFile(cfg.deployment(), content)
}

// replaceWatchNamespaceValue replaces the WATCH_NAMESPACE env value with chart.watchNamespace helper.
func replaceWatchNamespaceValue(cfg Config) error {
	content, err := readFile(cfg.deployment())
	if err != nil || content == "" {
		return err
	}
	if !strings.Contains(content, "WATCH_NAMESPACE") {
		return nil
	}
	if strings.Contains(content, `chart.watchNamespace`) {
		return nil // already patched
	}
	content = replaceEnvValue(content, "WATCH_NAMESPACE", `{{ include "chart.watchNamespace" . | quote }}`)
	return writeFile(cfg.deployment(), content)
}

// templateAppCredentialsEnv templates the APP_CREDENTIALS_SECRET_NAMESPACE env var value.
func templateAppCredentialsEnv(cfg Config) error {
	content, err := readFile(cfg.deployment())
	if err != nil || content == "" {
		return err
	}
	if strings.Contains(content, "controllerManager.appCredentialsSecretNamespace") {
		return nil // already patched
	}
	if !strings.Contains(content, "APP_CREDENTIALS_SECRET_NAMESPACE") {
		return nil
	}
	templateValue := `{{ .Values.controllerManager.appCredentialsSecretNamespace | default .Release.Namespace | quote }}`
	// Handle helmify's multi-line value pattern: replaces value line + optional continuation line
	content = replaceEnvValueMultiLine(content, "APP_CREDENTIALS_SECRET_NAMESPACE", templateValue)
	return writeFile(cfg.deployment(), content)
}

// --- ServiceAccount patches ---

// patchServiceAccountSecrets adds secrets block to serviceaccount template.
func patchServiceAccountSecrets(cfg Config) error {
	content, err := readFile(cfg.serviceAccount())
	if err != nil || content == "" {
		return err
	}
	if strings.Contains(content, "secrets") {
		return nil
	}
	insertion := `{{- with .Values.serviceAccount.secrets }}
secrets:
  {{- toYaml . | nindent 2 }}
{{- end }}`
	content = insertBeforeLine(content, "automountServiceAccountToken:", insertion)
	return writeFile(cfg.serviceAccount(), content)
}

// patchServiceAccountLabels adds custom labels support to serviceaccount template.
func patchServiceAccountLabels(cfg Config) error {
	content, err := readFile(cfg.serviceAccount())
	if err != nil || content == "" {
		return err
	}
	if strings.Contains(content, "serviceAccount.labels") {
		return nil
	}
	insertion := `  {{- with .Values.serviceAccount.labels }}
  {{- toYaml . | nindent 4 }}
  {{- end }}`
	content = insertAfterLine(content, `{{- include "chart.labels" . | nindent 4 }}`, insertion)
	return writeFile(cfg.serviceAccount(), content)
}

// --- RBAC patches ---

// copyManagerRBACTemplate copies the multi-namespace manager RBAC template.
// Only requires the source to exist; the destination is created if missing.
func copyManagerRBACTemplate(cfg Config) error {
	if _, err := os.Stat(cfg.managerRBACSrc()); os.IsNotExist(err) {
		return nil
	}
	return copyFile(cfg.managerRBACSrc(), cfg.managerRBAC())
}

// patchAppCredentialsRBAC adds namespace template to app-credentials Role and RoleBinding.
func patchAppCredentialsRBAC(cfg Config) error {
	content, err := readFile(cfg.appCredsRBAC())
	if err != nil || content == "" {
		return err
	}
	nsLine := `  namespace: {{ .Values.controllerManager.appCredentialsSecretNamespace | default .Release.Namespace }}`
	// Remove existing templated namespace lines
	content = removeLineContaining(content, "appCredentialsSecretNamespace")
	// Insert namespace after ALL metadata: lines
	content = insertAfterAllLines(content, "metadata:", nsLine)
	return writeFile(cfg.appCredsRBAC(), content)
}

// --- Webhook patches ---

// templateWebhookNamespaceSelector replaces hardcoded namespaceSelector with the helper.
func templateWebhookNamespaceSelector(cfg Config) error {
	content, err := readFile(cfg.webhook())
	if err != nil || content == "" {
		return err
	}
	if strings.Contains(content, "chart.webhookNamespaceSelector") {
		return nil // already patched
	}
	// Replace namespaceSelector block (3 lines: namespaceSelector + matchLabels + key/value)
	re := regexp.MustCompile(`(?m)  namespaceSelector:\n\s+matchLabels:\n\s+kubernetes\.io/metadata\.name:.*`)
	replacement := `  {{- include "chart.webhookNamespaceSelector" . | nindent 2 }}`
	content = re.ReplaceAllString(content, replacement)
	return writeFile(cfg.webhook(), content)
}

// --- PodDisruptionBudget patches ---

// copyPDBTemplate copies the values-driven PDB template from config/tmp/.
// Only requires the source to exist; the destination is created if missing.
func copyPDBTemplate(cfg Config) error {
	if _, err := os.Stat(cfg.pdbSrc()); os.IsNotExist(err) {
		return nil
	}
	return copyFile(cfg.pdbSrc(), cfg.pdb())
}

// addPDBValues adds podDisruptionBudget defaults to values.yaml if not present.
func addPDBValues(cfg Config) error {
	content, err := readFile(cfg.values())
	if err != nil || content == "" {
		return err
	}
	if strings.Contains(content, "podDisruptionBudget") {
		return nil
	}
	pdbBlock := "  podDisruptionBudget:\n    enabled: true\n    minAvailable: 1\n    # maxUnavailable: 1"
	content = insertAfterLine(content, "replicas:", pdbBlock)
	return writeFile(cfg.values(), content)
}

// addTopologySpreadValues adds default topologySpreadConstraints to values.yaml if not present.
// This ensures replicas are distributed across nodes to prevent PDB violations during node drains.
func addTopologySpreadValues(cfg Config) error {
	content, err := readFile(cfg.values())
	if err != nil || content == "" {
		return err
	}
	if strings.Contains(content, "topologyKey: kubernetes.io/hostname") {
		return nil
	}
	// Replace empty topologySpreadConstraints with the actual constraint
	old := "  topologySpreadConstraints: []"
	replacement := "  topologySpreadConstraints:\n" +
		"    - maxSkew: 1\n" +
		"      topologyKey: kubernetes.io/hostname\n" +
		"      whenUnsatisfiable: ScheduleAnyway"
	if strings.Contains(content, old) {
		content = strings.Replace(content, old, replacement, 1)
	} else if !strings.Contains(content, "topologySpreadConstraints") {
		content = insertAfterLine(content, "tolerations:", replacement)
	}
	return writeFile(cfg.values(), content)
}

// patchDeploymentTopologySpread replaces helmify's raw toYaml topologySpreadConstraints
// with a template that renders all user-provided fields via toYaml (using Helm's "omit"
// to exclude labelSelector), then injects a default labelSelector using chart.selectorLabels
// only when the user has not explicitly provided one.
func patchDeploymentTopologySpread(cfg Config) error {
	content, err := readFile(cfg.deployment())
	if err != nil || content == "" {
		return err
	}
	if !strings.Contains(content, "toYaml .Values.controllerManager.topologySpreadConstraints") {
		return nil // already patched or no raw toYaml block to replace
	}
	// Match helmify-generated pattern: "      topologySpreadConstraints: {{- toYaml ... }}"
	//nolint:lll
	re := regexp.MustCompile(
		`(?m)^\s+topologySpreadConstraints:.*toYaml .Values\.controllerManager\.topologySpreadConstraints[^\n]*\n(\s+\| nindent 8 \}\}\n)?`,
	)
	replacement := "      {{- with .Values.controllerManager.topologySpreadConstraints }}\n" +
		"      topologySpreadConstraints:\n" +
		"        {{- range . }}\n" +
		"        - {{- toYaml (omit . \"labelSelector\") | nindent 10 }}\n" +
		"          {{- if .labelSelector }}\n" +
		"          labelSelector: {{- toYaml .labelSelector | nindent 12 }}\n" +
		"          {{- else }}\n" +
		"          labelSelector:\n" +
		"            matchLabels:\n" +
		"              control-plane: controller-manager\n" +
		"              {{- include \"chart.selectorLabels\" $ | nindent 14 }}\n" +
		"          {{- end }}\n" +
		"        {{- end }}\n" +
		"      {{- end }}\n"
	content = re.ReplaceAllString(content, replacement)
	return writeFile(cfg.deployment(), content)
}

// --- Values.yaml patches ---

// patchValuesNamespaces transforms values.yaml namespace configuration:
// - Removes watchNamespace and appCredentialsSecretNamespace from env section
// - Removes empty env: key
// - Adds controllerManager.watchedNamespaces list
func patchValuesNamespaces(cfg Config) error {
	content, err := readFile(cfg.values())
	if err != nil || content == "" {
		return err
	}
	// Remove watchNamespace line
	content = removeLineContaining(content, "watchNamespace")
	// Remove appCredentialsSecretNamespace line
	content = removeLineContaining(content, "appCredentialsSecretNamespace")
	// Remove empty env: lines (with only whitespace after colon)
	re := regexp.MustCompile(`(?m)^\s*env:\s*$\n`)
	content = re.ReplaceAllString(content, "")
	// Add watchedNamespaces list if not present
	if !strings.Contains(content, "watchedNamespaces") {
		content = insertAfterLine(content, "controllerManager:", "  watchedNamespaces:\n    - github-configuration")
	}
	return writeFile(cfg.values(), content)
}

// patchValuesDefaults adds default values for podLabels, secrets, and labels.
func patchValuesDefaults(cfg Config) error {
	content, err := readFile(cfg.values())
	if err != nil || content == "" {
		return err
	}
	if !strings.Contains(content, "podLabels") {
		content = insertAfterLine(content, "controllerManager:", "  podLabels: {}")
	}
	if !strings.Contains(content, "secrets") {
		content = insertAfterLine(content, "serviceAccount:", "  secrets: []")
	}
	// Check if labels: exists under serviceAccount context
	if !hasKeyUnderSection(content, "serviceAccount:", "labels:") {
		content = insertAfterLine(content, "serviceAccount:", "  labels: {}")
	}
	return writeFile(cfg.values(), content)
}

// copyServingCertTemplate copies the values-driven serving-cert template.
// Only requires the source to exist; the destination is created if missing.
func copyServingCertTemplate(cfg Config) error {
	dst := cfg.ChartPath + "/templates/serving-cert.yaml"
	if _, err := os.Stat(cfg.servingCertSrc()); os.IsNotExist(err) {
		return nil
	}
	return copyFile(cfg.servingCertSrc(), dst)
}

// addServingCertValues adds servingCert defaults to values.yaml if not present.
func addServingCertValues(cfg Config) error {
	content, err := readFile(cfg.values())
	if err != nil || content == "" {
		return err
	}
	if strings.Contains(content, "servingCert") {
		return nil
	}
	content += `servingCert:
  # duration: 2160h0m0s
  issuerRef:
    kind: Issuer
    name: selfsigned-issuer
  # privateKey:
  #   algorithm: RSA
  #   size: 4096
  # renewBefore: 360h0m0s
  # subject:
  #   organizations:
  #     - My Organization
  # usages:
  #   - server auth
  #   - client auth
`
	return writeFile(cfg.values(), content)
}

// --- String manipulation helpers ---

// insertAfterLine inserts text after the FIRST line containing marker.
func insertAfterLine(content, marker, insertion string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, marker) {
			result := make([]string, 0, len(lines)+1)
			result = append(result, lines[:i+1]...)
			result = append(result, insertion)
			result = append(result, lines[i+1:]...)
			return strings.Join(result, "\n")
		}
	}
	return content
}

// insertAfterAllLines inserts text after EVERY line containing marker.
func insertAfterAllLines(content, marker, insertion string) string {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines)*2)
	for _, line := range lines {
		result = append(result, line)
		if strings.Contains(line, marker) {
			result = append(result, insertion)
		}
	}
	return strings.Join(result, "\n")
}

// insertBeforeLine inserts text before the FIRST line containing marker.
func insertBeforeLine(content, marker, insertion string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, marker) {
			result := make([]string, 0, len(lines)+1)
			result = append(result, lines[:i]...)
			result = append(result, insertion)
			result = append(result, lines[i:]...)
			return strings.Join(result, "\n")
		}
	}
	return content
}

// removeLineContaining removes all lines containing the given substring.
func removeLineContaining(content, substr string) string {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if !strings.Contains(line, substr) {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

// replaceEnvValue replaces the value line following a `- name: ENV_NAME` line.
func replaceEnvValue(content, envName, newValue string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, "name: "+envName) && i+1 < len(lines) {
			// Find the indentation of the value line
			nextLine := lines[i+1]
			indent := len(nextLine) - len(strings.TrimLeft(nextLine, " "))
			lines[i+1] = strings.Repeat(" ", indent) + "value: " + newValue
			break
		}
	}
	return strings.Join(lines, "\n")
}

// replaceEnvValueMultiLine replaces the value line(s) following a `- name: ENV_NAME` line.
// Handles helmify's multi-line value pattern where the expression wraps to the next line
// (e.g., "value: {{ quote .Values.foo\n  }}").
func replaceEnvValueMultiLine(content, envName, newValue string) string {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		if strings.Contains(lines[i], "name: "+envName) && i+1 < len(lines) {
			result = append(result, lines[i])
			// Determine indentation from the value line
			nextLine := lines[i+1]
			indent := len(nextLine) - len(strings.TrimLeft(nextLine, " "))
			result = append(result, strings.Repeat(" ", indent)+"value: "+newValue)
			i++ // skip the original value line
			// Skip continuation lines (lines that are just closing braces like "  }}")
			for i+1 < len(lines) {
				candidate := strings.TrimSpace(lines[i+1])
				if candidate == "}}" || candidate == "| default .Chart.AppVersion }}" {
					i++
				} else {
					break
				}
			}
		} else {
			result = append(result, lines[i])
		}
	}
	return strings.Join(result, "\n")
}

// hasKeyUnderSection checks if a key exists within a few lines after a section marker.
func hasKeyUnderSection(content, section, key string) bool {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, section) {
			// Check next 5 lines for the key
			end := min(i+5, len(lines))
			for _, l := range lines[i+1 : end] {
				if strings.Contains(l, key) {
					return true
				}
			}
			return false
		}
	}
	return false
}
