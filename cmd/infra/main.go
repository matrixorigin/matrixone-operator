// Copyright 2021 Matrix Origin
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
	"log"

	"github.com/matrixorigin/matrixone-operator/pkg/infra/aliyun"
	"github.com/matrixorigin/matrixone-operator/pkg/infra/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		pf := cfg.Get("installPlatform")

		switch pf {
		case "eks":
			aws.EKSDeploy(ctx, cfg)
		case "acs":
			aliyun.AliyunCSDeploy(ctx, cfg)
		default:
			log.Fatal("Please config your install platform!!!")
		}

		return nil
	})
}
