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

package main

import (
	"context"
	"github.com/matrixorigin/matrixone-operator/pkg/querycli"
	"go.uber.org/zap"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"time"
)

func main() {
	zapLogger := logzap.NewRaw()
	qc, err := querycli.New()
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := qc.ShowProcessList(ctx, "localhost:6004")
	if err != nil {
		panic(err)
	}
	zapLogger.Info("resp", zap.Any("resp", resp))
}
