package fake

import (
	"context"
	"github.com/go-logr/logr"
	recon "github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kubefake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func NewContext[T client.Object](obj T, c client.Client, emitter recon.EventEmitter) *recon.Context[T] {
	return &recon.Context[T]{
		Context: context.Background(),
		Obj:     obj,
		Client:  c,
		Log:     logr.Discard(),
		Event:   emitter,
	}
}

var KubeClientBuilder = kubefake.NewClientBuilder
