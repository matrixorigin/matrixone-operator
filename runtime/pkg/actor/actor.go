package actor

import (
	"context"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Actor[T client.Object] interface {
	Observe(*Context[T]) (Action[T], error)
	Delete(*Context[T]) error
}

type Context[T client.Object] struct {
	context.Context

	Obj T
}

type Action[T client.Object] func(*Context[T]) error