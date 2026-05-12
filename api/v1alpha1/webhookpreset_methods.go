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

func (p *WebhookPreset) SetSecretValue(secret string) {
	if secret == "" {
		return
	}
	p.Spec.SecretValue = &secret
}

func (p *WebhookPreset) GetSecretValueHash() string {
	if p.Spec.SecretValue == nil || *p.Spec.SecretValue == "" {
		return ""
	}
	// Use SHA-256 to hash the secret
	hash := sha256.Sum256([]byte(*p.Spec.SecretValue))
	hashValue := fmt.Sprintf("%x", hash)
	return hashValue
}
