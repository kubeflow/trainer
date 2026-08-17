// Copyright 2019 The Kubeflow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package control

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientscheme "k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	utiltesting "k8s.io/client-go/util/testing"

	testutilv1 "github.com/kubeflow/training-operator/test_job/test_util/v1"
)

func TestCreateService(t *testing.T) {
	ns := metav1.NamespaceDefault
	body := runtime.EncodeOrDie(
		clientscheme.Codecs.LegacyCodec(v1.SchemeGroupVersion),
		&v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "empty_service"}})
	fakeHandler := utiltesting.FakeHandler{
		StatusCode:   200,
		ResponseBody: body,
	}
	testServer := httptest.NewServer(&fakeHandler)
	defer testServer.Close()
	clientset := clientset.NewForConfigOrDie(&restclient.Config{
		Host: testServer.URL,
		ContentConfig: restclient.ContentConfig{
			GroupVersion: &v1.SchemeGroupVersion,
			ContentType:  runtime.ContentTypeJSON,
		},
	})

	serviceControl := RealServiceControl{
		KubeClient: clientset,
		Recorder:   &record.FakeRecorder{},
	}

	testJob := testutilv1.NewTestJob(1)

	testName := "service-name"
	service := testutilv1.NewBaseService(testName, testJob, t)
	service.SetOwnerReferences([]metav1.OwnerReference{})

	// Make sure createReplica sends a POST to the apiserver with a pod from the controllers pod template
	err := serviceControl.CreateServices(ns, service, testJob)
	assert.NoError(t, err, "unexpected error: %v", err)

	expectedService := v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Labels:    testutilv1.GenLabels(testJob.Name),
			Name:      testName,
			Namespace: ns,
		},
	}
	fakeHandler.ValidateRequest(t,
		"/api/v1/namespaces/default/services", "POST", nil)
	var actualService = &v1.Service{}
	err = json.Unmarshal([]byte(fakeHandler.RequestBody), actualService)
	assert.NoError(t, err, "unexpected error: %v", err)
	assert.True(t, apiequality.Semantic.DeepDerivative(&expectedService, actualService),
		"Body: %s", fakeHandler.RequestBody)
}

func TestCreateServicesWithControllerRef(t *testing.T) {
	ns := metav1.NamespaceDefault
	body := runtime.EncodeOrDie(
		clientscheme.Codecs.LegacyCodec(v1.SchemeGroupVersion),
		&v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "empty_service"}})
	fakeHandler := utiltesting.FakeHandler{
		StatusCode:   200,
		ResponseBody: body,
	}
	testServer := httptest.NewServer(&fakeHandler)
	defer testServer.Close()
	clientset := clientset.NewForConfigOrDie(&restclient.Config{
		Host: testServer.URL,
		ContentConfig: restclient.ContentConfig{
			GroupVersion: &v1.SchemeGroupVersion,
			ContentType:  runtime.ContentTypeJSON,
		},
	})

	serviceControl := RealServiceControl{
		KubeClient: clientset,
		Recorder:   &record.FakeRecorder{},
	}

	testJob := testutilv1.NewTestJob(1)

	testName := "service-name"
	service := testutilv1.NewBaseService(testName, testJob, t)
	service.SetOwnerReferences([]metav1.OwnerReference{})

	ownerRef := testutilv1.GenOwnerReference(testJob)

	// Make sure createReplica sends a POST to the apiserver with a pod from the controllers pod template
	err := serviceControl.CreateServicesWithControllerRef(ns, service, testJob, ownerRef)
	assert.NoError(t, err, "unexpected error: %v", err)

	expectedService := v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Labels:          testutilv1.GenLabels(testJob.Name),
			Name:            testName,
			Namespace:       ns,
			OwnerReferences: []metav1.OwnerReference{*ownerRef},
		},
	}
	fakeHandler.ValidateRequest(t,
		"/api/v1/namespaces/default/services", "POST", nil)
	var actualService = &v1.Service{}
	err = json.Unmarshal([]byte(fakeHandler.RequestBody), actualService)
	assert.NoError(t, err, "unexpected error: %v", err)
	assert.True(t, apiequality.Semantic.DeepDerivative(&expectedService, actualService),
		"Body: %s", fakeHandler.RequestBody)
}
