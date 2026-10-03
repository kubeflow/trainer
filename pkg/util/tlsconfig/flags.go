/*
Copyright The Kubeflow Authors.

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

package tlsconfig

import (
	"crypto/tls"
	"fmt"
	"strconv"
	"strings"
)

// FromFlags parses the generic TLS flags used by the controller manager and
// returns an option that can be applied to every TLS endpoint. Empty flags do
// not override the configuration file. Cipher suites use Go names and curve
// preferences use numeric Go tls.CurveID values separated by commas.
// configMinVersion is used to validate flag values against the effective
// minimum version when the command line does not override it.
func FromFlags(configMinVersion, minVersion, cipherSuites, curvePreferences string) (func(*tls.Config), error) {
	version, err := parseFlagTLSVersion(minVersion)
	if err != nil {
		return nil, err
	}
	effectiveVersion := version
	if effectiveVersion == 0 {
		effectiveVersion, err = parseFlagTLSVersion(configMinVersion)
		if err != nil {
			return nil, fmt.Errorf("invalid configured TLS minimum version: %w", err)
		}
	}
	if effectiveVersion == 0 {
		effectiveVersion = tls.VersionTLS12
	}
	ciphers, err := parseFlagCipherSuites(cipherSuites)
	if err != nil {
		return nil, err
	}
	curves, err := parseFlagCurvePreferences(curvePreferences)
	if err != nil {
		return nil, err
	}
	if version == 0 && ciphers == nil && curves == nil {
		return nil, nil
	}
	if effectiveVersion == tls.VersionTLS13 && ciphers != nil {
		return nil, fmt.Errorf("TLS cipher suites cannot be configured with TLS 1.3")
	}

	return func(config *tls.Config) {
		if version != 0 {
			config.MinVersion = version
		}
		if ciphers != nil {
			config.CipherSuites = ciphers
		}
		if curves != nil {
			config.CurvePreferences = curves
		}
	}, nil
}

func parseFlagTLSVersion(value string) (uint16, error) {
	switch strings.TrimSpace(value) {
	case "":
		return 0, nil
	case "VersionTLS12", "TLS1.2", "1.2":
		return tls.VersionTLS12, nil
	case "VersionTLS13", "TLS1.3", "1.3":
		return tls.VersionTLS13, nil
	default:
		return 0, fmt.Errorf("unsupported TLS minimum version %q", value)
	}
}

func parseFlagCipherSuites(value string) ([]uint16, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	known := make(map[string]uint16, len(tls.CipherSuites()))
	for _, suite := range tls.CipherSuites() {
		known[suite.Name] = suite.ID
	}

	result := make([]uint16, 0)
	for _, name := range strings.Split(value, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		id, ok := known[name]
		if !ok {
			return nil, fmt.Errorf("unsupported TLS cipher suite %q", name)
		}
		result = append(result, id)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("TLS cipher suite list is empty")
	}
	return result, nil
}

func parseFlagCurvePreferences(value string) ([]tls.CurveID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	result := make([]tls.CurveID, 0)
	for _, value := range strings.Split(value, ",") {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		id, err := strconv.ParseUint(value, 0, 16)
		if err != nil {
			return nil, fmt.Errorf("unsupported TLS curve preference %q: %w", value, err)
		}
		curve := tls.CurveID(id)
		if !supportedCurveIDs[curve] {
			return nil, fmt.Errorf("unsupported TLS curve preference %q", value)
		}
		result = append(result, curve)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("TLS curve preference list is empty")
	}
	return result, nil
}

var supportedCurveIDs = map[tls.CurveID]bool{
	tls.CurveP256:          true,
	tls.CurveP384:          true,
	tls.CurveP521:          true,
	tls.X25519:             true,
	tls.X25519MLKEM768:     true,
	tls.SecP256r1MLKEM768:  true,
	tls.SecP384r1MLKEM1024: true,
}
