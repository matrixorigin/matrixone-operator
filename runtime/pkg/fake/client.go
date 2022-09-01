package fake

import (
	"context"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ client.Client = &Client{}

// Client proxy the real client.Client and enables us to mock individual funcs
// when necessary
type Client struct {
	Client          client.Client
	MockGet         MockGetFn
	MockList        MockListFn
	MockCreate      MockCreateFn
	MockDelete      MockDeleteFn
	MockDeleteAllOf MockDeleteAllOfFn
	MockUpdate      MockUpdateFn
	MockPatch       MockPatchFn
}

func NewClient(c client.Client) *Client {
	return &Client{
		Client: c,
	}
}

func (c *Client) Status() client.StatusWriter {
	return c.Client.Status()
}

func (c *Client) Scheme() *runtime.Scheme {
	return c.Client.Scheme()
}

func (c *Client) RESTMapper() meta.RESTMapper {
	return c.Client.RESTMapper()
}

func (c *Client) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	if c.MockGet != nil {
		return c.MockGet(ctx, key, obj)
	}
	return c.Client.Get(ctx, key, obj)
}

func (c *Client) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if c.MockList != nil {
		return c.MockList(ctx, list, opts...)
	}
	return c.Client.List(ctx, list, opts...)
}

func (c *Client) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if c.MockCreate != nil {
		return c.MockCreate(ctx, obj, opts...)
	}
	return c.Client.Create(ctx, obj, opts...)
}

func (c *Client) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if c.MockDelete != nil {
		return c.MockDelete(ctx, obj, opts...)
	}
	return c.Client.Delete(ctx, obj, opts...)
}

func (c *Client) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	if c.MockDeleteAllOf != nil {
		return c.MockDeleteAllOf(ctx, obj, opts...)
	}
	return c.Client.DeleteAllOf(ctx, obj, opts...)
}

func (c *Client) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if c.MockUpdate != nil {
		return c.MockUpdate(ctx, obj, opts...)
	}
	return c.Client.Update(ctx, obj, opts...)
}

func (c *Client) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return c.MockPatch(ctx, obj, patch, opts...)
}

type MockGetFn func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error

type MockListFn func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error

type MockCreateFn func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error

type MockDeleteFn func(ctx context.Context, obj runtime.Object, opts ...client.DeleteOption) error

type MockDeleteAllOfFn func(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error

type MockUpdateFn func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error

type MockPatchFn func(ctx context.Context, obj runtime.Object, patch client.Patch, opts ...client.PatchOption) error

type MockStatusUpdateFn func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error

type MockStatusPatchFn func(ctx context.Context, obj runtime.Object, patch client.Patch, opts ...client.PatchOption) error

type ObjectFn func(obj runtime.Object) error

func NewMockGetFn(err error, ofn ...ObjectFn) MockGetFn {
	return func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
		for _, fn := range ofn {
			if err := fn(obj); err != nil {
				return err
			}
		}
		return err
	}
}

func NewMockListFn(err error, ofn ...ObjectFn) MockListFn {
	return func(_ context.Context, obj runtime.Object, _ ...client.ListOption) error {
		for _, fn := range ofn {
			if err := fn(obj); err != nil {
				return err
			}
		}
		return err
	}
}

func NewMockCreateFn(err error, ofn ...ObjectFn) MockCreateFn {
	return func(_ context.Context, obj runtime.Object, opts ...client.CreateOption) error {
		for _, fn := range ofn {
			if err := fn(obj); err != nil {
				return err
			}
		}
		return err
	}
}

func NewMockDeleteFn(err error, ofn ...ObjectFn) MockDeleteFn {
	return func(_ context.Context, obj runtime.Object, _ ...client.DeleteOption) error {
		for _, fn := range ofn {
			if err := fn(obj); err != nil {
				return err
			}
		}
		return err
	}
}

func NewMockDeleteAllOfFn(err error, ofn ...ObjectFn) MockDeleteAllOfFn {
	return func(_ context.Context, obj runtime.Object, _ ...client.DeleteAllOfOption) error {
		for _, fn := range ofn {
			if err := fn(obj); err != nil {
				return err
			}
		}
		return err
	}
}

func NewMockUpdateFn(err error, ofn ...ObjectFn) MockUpdateFn {
	return func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
		for _, fn := range ofn {
			if err := fn(obj); err != nil {
				return err
			}
		}
		return err
	}
}

func NewMockPatchFn(err error, ofn ...ObjectFn) MockPatchFn {
	return func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) error {
		for _, fn := range ofn {
			if err := fn(obj); err != nil {
				return err
			}
		}
		return err
	}
}
