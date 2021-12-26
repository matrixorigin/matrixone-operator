package controllers

import "sigs.k8s.io/controller-runtime/pkg/predicate"

type GenericPredicates struct {
	predicate.Funcs
}
