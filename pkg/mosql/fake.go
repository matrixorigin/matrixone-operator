// Copyright 2024 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mosql

import (
	"context"
	"database/sql"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewFakeClient(_ string, _ client.Client, _ types.NamespacedName) Client {
	return &fakeClient{}
}

type fakeClient struct{}

func (c *fakeClient) GetServerConnection(ctx context.Context, uid string) (int, error) {
	return 0, nil
}

func (c *fakeClient) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return nil, nil
}
