package k8sutils

import (
	"context"
	"sort"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	MatrixoneExporterContainer = "matrixone-exporter"
	graceTime                  = 15
)

type containerParameters struct {
	Image                            string
	ImagePullPolicy                  corev1.PullPolicy
	Resources                        *corev1.ResourceRequirements
	MatrixoneExporterImage           string
	MatrixoneExporterImagePullPolicy corev1.PullPolicy
	MatrixoneExporterResources       *corev1.ResourceRequirements
	MatrixoneExporterEnv             *[]corev1.EnvVar
	Role                             string
	EnabledPassword                  *bool
	SecretName                       *string
	SecretKey                        *string
	PersistenceEnabled               *bool
	ReadinessProbe                   *corev1.Probe
	LivenessProbe                    *corev1.Probe
}

type statefulSetParameters struct {
	Replicas              *int32
	NodeSelector          map[string]string
	SecurityContext       *corev1.PodSecurityContext
	PriorityClassName     string
	Affinity              *corev1.Affinity
	Tolerations           *[]corev1.Toleration
	EnableMetrics         bool
	PersistentVolumeClaim corev1.PersistentVolumeClaim
	ImagePullSecrets      *[]corev1.LocalObjectReference
	ExternalConfig        *string
}

func stateFulSetLogger(namespace string, name string) logr.Logger {
	reqLogger := log.WithValues("Request.StateFulSet.Namespace", namespace, "Request.StateFulSet.Name", name)
	return reqLogger
}

func CreateOrUpdateStateFul(namespace string, stsMeta metav1.ObjectMeta, labels map[string]string, params statefulSetParameters, ownerDef metav1.OwnerReference, containerParams containerParameters) error {
	logger := stateFulSetLogger(namespace, stsMeta.Name)
	storedStateful, err := GetStateFulSet(namespace, stsMeta.Name)
	statefulSetDef := generateStateFulSetsDef(stsMeta, labels, params, ownerDef, containerParams)
	if err != nil {
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(statefulSetDef); err != nil {
			logger.Error(err, "Unable to patch matrixone statefulset with comparison object")
			return err
		}
		if errors.IsNotFound(err) {
			return createStateFulSet(namespace, statefulSetDef)
		}
		return err
	}
	return patchStateFulSet(storedStateful, statefulSetDef, namespace)
}

func GetStateFulSet(namespace string, stateful string) (*appsv1.StatefulSet, error) {
	logger := stateFulSetLogger(namespace, stateful)
	statefulInfo, err := generateK8sClient().AppsV1().StatefulSets(namespace).Get(context.TODO(), stateful, metav1.GetOptions{})
	if err != nil {
		logger.Info("Matrixone statefulset get action failed")
		return nil, err
	}
	logger.Info("Matrixone statefulset get action was successful")
	return statefulInfo, err
}

func generateStateFulSetsDef(stsMeta metav1.ObjectMeta, labels map[string]string, params statefulSetParameters, ownerDef metav1.OwnerReference, containerParams containerParameters) *appsv1.StatefulSet {
	statefulset := &appsv1.StatefulSet{
		TypeMeta:   generateMetaInformation("StatefulSet", "apps/v1"),
		ObjectMeta: stsMeta,
		Spec: appsv1.StatefulSetSpec{
			Selector:    LabelSelectors(labels),
			ServiceName: stsMeta.Name,
			Replicas:    params.Replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers:        generateContainerDef(stsMeta.Name, containerParams, params.EnableMetrics, params.ExternalConfig),
					NodeSelector:      params.NodeSelector,
					SecurityContext:   params.SecurityContext,
					PriorityClassName: params.PriorityClassName,
					Affinity:          params.Affinity,
				},
			},
		},
	}
	if params.Tolerations != nil {
		statefulset.Spec.Template.Spec.Tolerations = *params.Tolerations
	}
	if params.ImagePullSecrets != nil {
		statefulset.Spec.Template.Spec.ImagePullSecrets = *params.ImagePullSecrets
	}
	if containerParams.PersistenceEnabled != nil && *containerParams.PersistenceEnabled {
		statefulset.Spec.VolumeClaimTemplates = append(statefulset.Spec.VolumeClaimTemplates, createPVCTemplate(stsMeta.Name, params.PersistentVolumeClaim))
	}
	if params.ExternalConfig != nil {
		statefulset.Spec.Template.Spec.Volumes = getExternalConfig(*params.ExternalConfig)
	}
	AddOwnerRefToObject(statefulset, ownerDef)
	return statefulset
}

func getExternalConfig(configMapName string) []corev1.Volume {
	return []corev1.Volume{
		{
			Name: "external-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapName,
					},
				},
			},
		},
	}
}

func createPVCTemplate(name string, storageSpec corev1.PersistentVolumeClaim) corev1.PersistentVolumeClaim {
	pvcTemplate := storageSpec
	pvcTemplate.CreationTimestamp = metav1.Time{}
	pvcTemplate.Name = name
	if storageSpec.Spec.AccessModes == nil {
		pvcTemplate.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	} else {
		pvcTemplate.Spec.AccessModes = storageSpec.Spec.AccessModes
	}
	pvcTemplate.Spec.Resources = storageSpec.Spec.Resources
	pvcTemplate.Spec.Selector = storageSpec.Spec.Selector
	pvcTemplate.Spec.Selector = storageSpec.Spec.Selector
	return pvcTemplate
}

func createStateFulSet(namespace string, stateful *appsv1.StatefulSet) error {
	logger := stateFulSetLogger(namespace, stateful.Name)
	_, err := generateK8sClient().AppsV1().StatefulSets(namespace).Create(context.TODO(), stateful, metav1.CreateOptions{})
	if err != nil {
		logger.Error(err, "Matrixone stateful creation failed")
		return err
	}
	logger.Info("Matrixone stateful successfully created")
	return nil
}

func updateStateFulSet(namespace string, stateful *appsv1.StatefulSet) error {
	logger := stateFulSetLogger(namespace, stateful.Name)
	_, err := generateK8sClient().AppsV1().StatefulSets(namespace).Update(context.TODO(), stateful, metav1.UpdateOptions{})
	if err != nil {
		logger.Error(err, "Matrixone stateful update failed")
		return err
	}
	logger.Info("Matrixone stateful ")
	return nil
}

func patchStateFulSet(storedStateful *appsv1.StatefulSet, newStateful *appsv1.StatefulSet, namespace string) error {
	logger := stateFulSetLogger(namespace, storedStateful.Name)
	patchResult, err := patch.DefaultPatchMaker.Calculate(storedStateful, newStateful)
	if err != nil {
		logger.Error(err, "Unable to patch matirxone statefulset with comparison object")
		return err
	}
	if !patchResult.IsEmpty() {
		newStateful.ResourceVersion = storedStateful.ResourceVersion
		newStateful.CreationTimestamp = storedStateful.CreationTimestamp
		newStateful.ManagedFields = storedStateful.ManagedFields
		for key, value := range storedStateful.Annotations {
			if _, present := newStateful.Annotations[key]; !present {
				newStateful.Annotations[key] = value
			}
		}
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(newStateful); err != nil {
			logger.Error(err, "Unable to patch matrixone statefulset with comparison object")
			return err
		}
		return updateStateFulSet(namespace, newStateful)
	}
	return nil
}

func generateContainerDef(name string, containerParams containerParameters, enableMetrics bool, externalConfig *string) []corev1.Container {
	containerDefinition := []corev1.Container{
		{
			Name:            name,
			Image:           containerParams.Image,
			ImagePullPolicy: containerParams.ImagePullPolicy,
			Env:             getEnvironmentVariables(containerParams.Role, containerParams.EnabledPassword, containerParams.SecretName, containerParams.SecretKey, containerParams.PersistenceEnabled, containerParams.MatrixoneExporterEnv),
			VolumeMounts:    getVolumeMount(name, containerParams.PersistenceEnabled, externalConfig),
		},
	}
	if containerParams.ReadinessProbe != nil {
		containerDefinition[0].ReadinessProbe = containerParams.ReadinessProbe
	} else {
		containerDefinition[0].ReadinessProbe = getProbeInfo()
	}
	if containerParams.LivenessProbe != nil {
		containerDefinition[0].LivenessProbe = containerParams.LivenessProbe
	} else {
		containerDefinition[0].LivenessProbe = getProbeInfo()
	}

	if containerParams.Resources != nil {
		containerDefinition[0].Resources = *containerParams.Resources
	}
	if enableMetrics {
		containerDefinition = append(containerDefinition, enableMatrixoneMonitoring(containerParams))
	}
	return containerDefinition
}

func enableMatrixoneMonitoring(params containerParameters) corev1.Container {
	exporterDefinition := corev1.Container{
		Name:            MatrixoneExporterContainer,
		Image:           params.MatrixoneExporterImage,
		ImagePullPolicy: params.MatrixoneExporterImagePullPolicy,
		Env:             getEnvironmentVariables(params.Role, params.EnabledPassword, params.SecretName, params.SecretKey, params.PersistenceEnabled, params.MatrixoneExporterEnv),
	}
	if params.MatrixoneExporterResources != nil {
		exporterDefinition.Resources = *params.MatrixoneExporterResources
	}
	return exporterDefinition
}

func getVolumeMount(name string, persistenceEnabled *bool, externalConfig *string) []corev1.VolumeMount {
	var volumeMounts []corev1.VolumeMount
	if persistenceEnabled != nil && *persistenceEnabled {
		volumeMounts = []corev1.VolumeMount{
			{
				Name:      name,
				MountPath: "/cube",
			},
		}
	}

	if externalConfig != nil {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "external-config",
			MountPath: "/config/config.toml",
		})
	}
	return volumeMounts
}

func getProbeInfo() *corev1.Probe {
	return &corev1.Probe{
		InitialDelaySeconds: graceTime,
		PeriodSeconds:       15,
		FailureThreshold:    5,
		TimeoutSeconds:      5,
		Handler: corev1.Handler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"bash",
					"/usr/bin/healthcheck.sh",
				},
			},
		},
	}
}

func getEnvironmentVariables(role string, enabledPassword *bool, secretName *string, secretKey *string, persistenceEnabled *bool, extraEnv *[]corev1.EnvVar) []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{Name: "SERVER_MODE", Value: role},
		{Name: "SETUP_MODE", Value: role},
		{Name: "Matrixone_ADDR", Value: "mysql://localhost:6001"},
	}
	if enabledPassword != nil && *enabledPassword {
		envVars = append(envVars, corev1.EnvVar{
			Name: "MATRIXONE_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: *secretName,
					},
					Key: *secretKey,
				},
			},
		})
	}
	if persistenceEnabled != nil && *persistenceEnabled {
		envVars = append(envVars, corev1.EnvVar{Name: "PERSISTENCE_ENABLED", Value: "true"})
	}

	if extraEnv != nil {
		envVars = append(envVars, *extraEnv...)
	}
	sort.SliceStable(envVars, func(i, j int) bool {
		return envVars[i].Name < envVars[j].Name
	})
	return envVars
}
