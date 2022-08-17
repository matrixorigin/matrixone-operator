package v1alpha1

import "fmt"

func (m *MatrixOneCluster) LogSetImage() string {
	image := m.Spec.LogService.Image
	if image == "" {
		image = m.defaultImage()
	}
	return image
}

func (m *MatrixOneCluster) DnSetImage() string {
	image := m.Spec.DN.Image
	if image == "" {
		image = m.defaultImage()
	}
	return image
}

func (m *MatrixOneCluster) TpSetImage() string {
	image := m.Spec.TP.Image
	if image == "" {
		image = m.defaultImage()
	}
	return image
}

func (m *MatrixOneCluster) ApSetImage() string {
	image := m.Spec.AP.Image
	if image == "" {
		image = m.defaultImage()
	}
	return image
}

func (m *MatrixOneCluster) defaultImage() string {
	return fmt.Sprintf("%s:%s", m.Spec.ImageRepository, m.Spec.Version)
}
