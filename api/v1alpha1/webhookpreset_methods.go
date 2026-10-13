/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"crypto/sha256"
	"fmt"
)

// SetSecretValue sets the (plaintext) secret value of the webhook. Empty values are ignored.
func (s *WebhookPresetSpec) SetSecretValue(secret string) {
	if secret == "" {
		return
	}
	s.SecretValue = &secret
}

// GetSecretValueHash returns the SHA-256 hex hash of the secret value, or "" if no secret value is set.
func (s *WebhookPresetSpec) GetSecretValueHash() string {
	if s.SecretValue == nil || *s.SecretValue == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(*s.SecretValue))
	return fmt.Sprintf("%x", hash)
}

func (p *WebhookPreset) SetSecretValue(secret string) {
	p.Spec.SetSecretValue(secret)
}

func (p *WebhookPreset) GetSecretValueHash() string {
	return p.Spec.GetSecretValueHash()
}
