package fake

// TODO(aylei): fake kubeClient for UT
func NewClient() *FakeKubeClient {
	return nil
}

type FakeKubeClient struct{}
