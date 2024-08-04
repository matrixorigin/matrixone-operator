package bucketclaim

import (
	corev1 "k8s.io/api/core/v1"
)

type Option func(actor *Actor)

func WithImage(image string) Option {
	return func(actor *Actor) {
		actor.image = image
	}
}

func WithImagePullSecrets(secrets []corev1.LocalObjectReference) Option {
	return func(actor *Actor) {
		if actor.imagePullSecrets == nil {
			actor.imagePullSecrets = make([]corev1.LocalObjectReference, 0, len(secrets))
		}
		actor.imagePullSecrets = append(actor.imagePullSecrets, secrets...)
	}
}
