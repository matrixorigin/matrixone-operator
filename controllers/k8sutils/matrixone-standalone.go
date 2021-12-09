package k8sutils

import matrixonev1alpha1 "github.com/matrixorigin/matrixone-operator/api/v1alpha1"

var (
	enableMetrics bool
)

func generateMatrixoneStandaloneParams(cr *matrixonev1alpha1.Matrixone) statefulSetParameters {
	replicas := int32(1)
	res := statefulSetParameters{
		Replicas:          &replicas,
		NodeSelector:      cr.Spec.NodeSelector,
		SecurityContext:   cr.Spec.SecurityContext,
		PriorityClassName: cr.Spec.PriorityClassName,
		Affinity:          cr.Spec.Affinity,
		Tolerations:       cr.Spec.Tolerations,
	}
	if cr.Spec.KubernetesConfig.ImagePullSecrets != nil {
		res.ImagePullSecrets = cr.Spec.KubernetesConfig.ImagePullSecrets
	}
	if cr.Spec.Storage != nil {
		res.PersistentVolumeClaim = cr.Spec.Storage.VolumeClaimTemplate
	}
	if cr.Spec.MatrixoneConfig != nil {
		res.ExternalConfig = cr.Spec.MatrixoneConfig.AdditionalMatrixoneConfig
	}
	if cr.Spec.MatrixoneExporter != nil {
		res.EnableMetrics = cr.Spec.MatrixoneExporter.Enabled

	}
	return res
}

func generateMatrixoneStandaloneContainerParams(cr *matrixonev1alpha1.Matrixone) containerParameters {
	trueProperty := true
	falseProperty := false
	containerProp := containerParameters{
		Role:            "standalone",
		Image:           cr.Spec.KubernetesConfig.Image,
		ImagePullPolicy: cr.Spec.KubernetesConfig.ImagePullPolicy,
		Resources:       cr.Spec.KubernetesConfig.Resources,
	}
	if cr.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
		containerProp.EnabledPassword = &trueProperty
		containerProp.SecretName = cr.Spec.KubernetesConfig.ExistingPasswordSecret.Name
		containerProp.SecretKey = cr.Spec.KubernetesConfig.ExistingPasswordSecret.Key
	} else {
		containerProp.EnabledPassword = &falseProperty
	}
	if cr.Spec.MatrixoneExporter != nil {
		containerProp.MatrixoneExporterImage = cr.Spec.MatrixoneExporter.Image
		containerProp.MatrixoneExporterImagePullPolicy = cr.Spec.MatrixoneExporter.ImagePullPolicy

		if cr.Spec.MatrixoneExporter.Resources != nil {
			containerProp.MatrixoneExporterResources = cr.Spec.MatrixoneExporter.Resources
		}

		if cr.Spec.MatrixoneExporter.EnvVars != nil {
			containerProp.MatrixoneExporterEnv = cr.Spec.MatrixoneExporter.EnvVars
		}

	}
	if cr.Spec.ReadinessProbe != nil {
		containerProp.ReadinessProbe = cr.Spec.ReadinessProbe
	}
	if cr.Spec.LivenessProbe != nil {
		containerProp.LivenessProbe = cr.Spec.LivenessProbe
	}
	if cr.Spec.Storage != nil {
		containerProp.PersistenceEnabled = &trueProperty
	}
	return containerProp
}

func CreateStandAloneMatrixone(cr *matrixonev1alpha1.Matrixone) error {
	logger := stateFulSetLogger(cr.Namespace, cr.ObjectMeta.Name)
	labels := getMatrixoneLabels(cr.ObjectMeta.Name, "standalone", "standalone")
	objectMetaInfo := generateObjectMetaInformation(cr.ObjectMeta.Name, cr.Namespace, labels, generateStatefulSetsAnots())
	err := CreateOrUpdateStateFul(cr.Namespace,
		objectMetaInfo,
		labels,
		generateMatrixoneStandaloneParams(cr),
		matrixoneAsOwner(cr),
		generateMatrixoneStandaloneContainerParams(cr),
	)
	if err != nil {
		logger.Error(err, "Cannot create standalone statefulset for Matrixone")
		return err
	}
	return nil
}

func CreateStandAloneService(cr *matrixonev1alpha1.Matrixone) error {
	logger := serviceLogger(cr.Namespace, cr.ObjectMeta.Name)
	labels := getMatrixoneLabels(cr.ObjectMeta.Name, "standalone", "standalone")
	if cr.Spec.MatrixoneExporter != nil && cr.Spec.MatrixoneExporter.Enabled {
		enableMetrics = true
	}
	objectMetaInfo := generateObjectMetaInformation(cr.ObjectMeta.Name, cr.Namespace, labels, generateServiceAnots())
	headlessObjectMetaInfo := generateObjectMetaInformation(cr.ObjectMeta.Name+"-headless", cr.Namespace, labels, generateServiceAnots())
	err := CreateOrUpdateHeadlessService(cr.Namespace, headlessObjectMetaInfo, labels, matrixoneAsOwner(cr))
	if err != nil {
		logger.Error(err, "Cannot create standalone headless service for Redis")
		return err
	}
	err = CreateOrUpdateService(cr.Namespace, objectMetaInfo, labels, matrixoneAsOwner(cr), enableMetrics)
	if err != nil {
		logger.Error(err, "Cannot create standalone service for Redis")
		return err
	}
	return nil
}

func getMatrixoneLabels(name, setupType, role string) map[string]string {
	return map[string]string{
		"app":                  name,
		"Matrixone_setup_type": setupType,
		"role":                 role,
	}
}
