package v1alpha1

import "fmt"

func (m *MatrixoneCluster) LogSetImage() string {
	image := m.Spec.LogService.Image
	if image == "" {
		image = m.defaultImage()
	}
	return image
}

func (m *MatrixoneCluster) DnSetImage() string {
	image := m.Spec.DN.Image
	if image == "" {
		image = m.defaultImage()
	}
	return image
}

func (m *MatrixoneCluster) TpSetImage() string {
	image := m.Spec.TP.Image
	if image == "" {
		image = m.defaultImage()
	}
	return image
}

func (m *MatrixoneCluster) ApSetImage() string {
	image := m.Spec.AP.Image
	if image == "" {
		image = m.defaultImage()
	}
	return image
}

func (m *MatrixoneCluster) defaultImage() string {
	return fmt.Sprintf("%s:%s", m.Spec.ImageRepository, m.Spec.Version)
}
