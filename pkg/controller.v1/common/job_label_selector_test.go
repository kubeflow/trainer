// Copyright 2021 The Kubeflow Authors
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

package common

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"testing"
)

func TestMatchesJobLabels(t *testing.T) {
	for _, tc := range []struct {
		name, selector string
		jobLabels      map[string]string
		want           bool
	}{
		{name: "default", want: true},
		{name: "equality", selector: "owner=new", jobLabels: map[string]string{"owner": "new"}, want: true},
		{name: "different", selector: "owner=new", jobLabels: map[string]string{"owner": "old"}},
		{name: "missing", selector: "owner=new"},
		{name: "set", selector: "owner in (new,canary)", jobLabels: map[string]string{"owner": "canary"}, want: true},
		{name: "exists", selector: "owner", jobLabels: map[string]string{"owner": ""}, want: true},
		{name: "absent", selector: "!owner", want: true},
		{name: "negative matches absent", selector: "owner!=old", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selector, err := labels.Parse(tc.selector)
			if err != nil {
				t.Fatal(err)
			}
			jc := &JobController{JobLabelSelector: selector}
			if got := jc.MatchesJobLabels(&metav1.ObjectMeta{Labels: tc.jobLabels}); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
	if !(&JobController{}).MatchesJobLabels(&metav1.ObjectMeta{}) {
		t.Fatal("nil selector must preserve default behavior")
	}
}
