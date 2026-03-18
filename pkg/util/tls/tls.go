/*
Copyright 2025 The Kubeflow Authors.

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

package tlsutil

import (
	"crypto/tls"

	"k8s.io/klog/v2"
)

// ParseTLSVersion converts a TLS version string to its uint16 constant.
// Logs a warning and returns 0 if the version is not recognized.
func ParseTLSVersion(version string) uint16 {
	switch version {
	case "1.0":
		return tls.VersionTLS10
	case "1.1":
		return tls.VersionTLS11
	case "1.2":
		return tls.VersionTLS12
	case "1.3":
		return tls.VersionTLS13
	default:
		klog.Warningf("Unrecognized TLS version %q, using Go default", version)
		return 0
	}
}

// ParseCipherSuiteIDs converts cipher suite name strings to their uint16 IDs.
// Unrecognized names are silently ignored. Insecure suites trigger a warning.
func ParseCipherSuiteIDs(names []string) []uint16 {
	secureLookup := make(map[string]uint16, len(tls.CipherSuites()))
	for _, cs := range tls.CipherSuites() {
		secureLookup[cs.Name] = cs.ID
	}
	insecureLookup := make(map[string]uint16, len(tls.InsecureCipherSuites()))
	for _, cs := range tls.InsecureCipherSuites() {
		insecureLookup[cs.Name] = cs.ID
	}
	ids := make([]uint16, 0, len(names))
	for _, name := range names {
		if id, ok := secureLookup[name]; ok {
			ids = append(ids, id)
		} else if id, ok := insecureLookup[name]; ok {
			klog.Warningf("Cipher suite %q is insecure and should not be used in production", name)
			ids = append(ids, id)
		}
	}
	return ids
}
