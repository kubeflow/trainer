/*
Copyright 2021.

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

package main

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"testing"
)

func TestInvalidJobLabelSelector(t *testing.T) {
	// Exercise main in a subprocess because invalid configuration calls os.Exit.
	cmd := exec.Command(os.Args[0], "-test.run=^TestInvalidJobLabelSelectorProcess$")
	cmd.Env = append(os.Environ(), "TRAINER_TEST_INVALID_SELECTOR=1")
	output, err := cmd.CombinedOutput()
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit 1, got %v: %s", err, output)
	}
	if !bytes.Contains(output, []byte("invalid --job-label-selector")) {
		t.Fatalf("missing selector validation error: %s", output)
	}
}

func TestInvalidJobLabelSelectorProcess(t *testing.T) {
	if os.Getenv("TRAINER_TEST_INVALID_SELECTOR") != "1" {
		return
	}
	flag.CommandLine = flag.NewFlagSet("training-operator", flag.ExitOnError)
	os.Args = []string{"training-operator", "--job-label-selector=owner in ("}
	main()
	t.Fatal("main returned for an invalid selector")
}
