// Package redact masks sensitive values (secrets) in deployment output.
//
// A Redactor is constructed from the deployment's environment and its list
// of secret key names. It replaces every occurrence of a secret's value
// with "***" in any string passed to Redact.
package redact

import "strings"

// Redactor replaces secret values with "***" in any string.
type Redactor struct {
	secrets []string // actual secret values to mask
}

// New creates a Redactor. It looks up each key in secretKeys from the env map
// and collects the non-empty values for masking.
func New(env map[string]string, secretKeys []string) *Redactor {
	var secrets []string
	for _, key := range secretKeys {
		if val, ok := env[key]; ok && val != "" {
			secrets = append(secrets, val)
		}
	}
	return &Redactor{secrets: secrets}
}

// Redact replaces all known secret values in s with "***".
func (r *Redactor) Redact(s string) string {
	for _, secret := range r.secrets {
		s = strings.ReplaceAll(s, secret, "***")
	}
	return s
}
