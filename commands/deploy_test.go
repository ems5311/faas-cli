// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package commands

import (
	"net/http"
	"os"
	"regexp"
	"testing"

	"github.com/openfaas/faas-cli/test"
)

func Test_deploy(t *testing.T) {
	s := test.MockHttpServer(t, []test.Request{
		{
			Method:             http.MethodPut,
			Uri:                "/system/functions",
			ResponseStatusCode: http.StatusOK,
		},
	})
	defer s.Close()

	stdOut := test.CaptureStdout(func() {
		faasCmd.SetArgs([]string{
			"deploy",
			"--gateway=" + s.URL,
			"--image=golang",
			"--name=test-function",
		})
		faasCmd.Execute()
	})

	regexStr := `(?m:Deploying: test-function.)`
	if found, err := regexp.MatchString(regexStr, stdOut); err != nil || !found {
		t.Fatalf("Tried to match regex '%s' but got: '%s'\n", regexStr, stdOut)
	}

	regexStr = `(?m:Deployed)`
	if found, err := regexp.MatchString(regexStr, stdOut); err != nil || !found {
		t.Fatalf("Tried to match regex '%s' but got: '%s'\n", regexStr, stdOut)
	}

	regexStr = `(?m:200 OK)`
	if found, err := regexp.MatchString(regexStr, stdOut); err != nil || !found {
		t.Fatalf("Tried to match regex '%s' but got: '%s'\n", regexStr, stdOut)
	}
}

func Test_deploy_stackYAML(t *testing.T) {
	s := test.MockHttpServer(t, []test.Request{
		{
			Method:             http.MethodPut,
			Uri:                "/system/functions",
			ResponseStatusCode: http.StatusOK,
		},
	})
	defer s.Close()

	funcName := "stack"
	yamlFile = "stack.yml"

	// Cleanup the created directory
	defer func() {
		os.RemoveAll(funcName)
		os.Remove(yamlFile)
	}()

	faasCmd.SetArgs([]string{
		"new",
		"--gateway=" + s.URL,
		"--lang=" + "Dockerfile",
		"stack",
	})
	faasCmd.Execute()

	stdOut := test.CaptureStdout(func() {
		faasCmd.SetArgs([]string{
			"deploy",
			"--gateway=" + s.URL,
			"--image=golang",
			"--name=test-function",
		})
		faasCmd.Execute()
	})

	regexStr := `(?m:Deploying: test-function.)`
	if found, err := regexp.MatchString(regexStr, stdOut); err != nil || !found {
		t.Fatalf("Tried to match regex '%s' but got: '%s'\n", regexStr, stdOut)
	}

	regexStr = `(?m:Deployed)`
	if found, err := regexp.MatchString(regexStr, stdOut); err != nil || !found {
		t.Fatalf("Tried to match regex '%s' but got: '%s'\n", regexStr, stdOut)
	}

	regexStr = `(?m:200 OK)`
	if found, err := regexp.MatchString(regexStr, stdOut); err != nil || !found {
		t.Fatalf("Tried to match regex '%s' but got: '%s'\n", regexStr, stdOut)
	}
}
