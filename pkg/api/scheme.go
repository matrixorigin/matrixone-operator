// Copyright 2021 Matrix Origin
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

package api

import (
	mocv1alpha1 "github.com/matrixorigin/matrixone-operator/pkg/apis/matrixone/v1alpha1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kscheme "k8s.io/client-go/kubernetes/scheme"
	apireg "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

var Scheme = runtime.NewScheme()
var Codecs = serializer.NewCodecFactory(Scheme)
var ParameterCodec = runtime.NewParameterCodec(Scheme)
var localSchemeBuilder = runtime.SchemeBuilder{
	apireg.AddToScheme,
	kscheme.AddToScheme,
	apiext.AddToScheme,
	mocv1alpha1.AddToScheme,
}

// AddToScheme adds the types in this group-version to the given scheme.
var AddToScheme = localSchemeBuilder.AddToScheme

func init() {
	// GroupVersion is group version used to register these objects
	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Group: "matrixone.matrixorigin.cn", Version: "v1alpha1"})

	utilruntime.Must(AddToScheme(Scheme))
}
