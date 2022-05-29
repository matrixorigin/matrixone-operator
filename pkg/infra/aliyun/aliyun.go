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

package aliyun

import (
	"fmt"
	"strconv"

	"github.com/pulumi/pulumi-alicloud/sdk/v3/go/alicloud"
	"github.com/pulumi/pulumi-alicloud/sdk/v3/go/alicloud/vpc"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func AliyunCSDeploy(ctx *pulumi.Context) error {

	fmt.Println("deploy to aliyun")

	aliZones, err := alicloud.GetZones(ctx, &alicloud.GetZonesArgs{
		AvailableResourceCreation: pulumi.StringRef("VSwitch"),
	}, nil)
	if err != nil {
		return nil
	}

	aliNetwork, err := vpc.NewNetwork(ctx, "mo-vpc", &vpc.NetworkArgs{
		CidrBlock: pulumi.String("172.16.0.0./24"),
	}, nil)
	if err != nil {
		return nil
	}

	var switchGroup []*vpc.Switch
	for k := range aliZones.Zones {
		dSwitch, err := vpc.NewSwitch(ctx, "mo-switch-"+strconv.Itoa(k), &vpc.SwitchArgs{
			VpcId:       aliNetwork.ID(),
			CidrBlock:   pulumi.String("172.16.0.0/24"),
			ZoneId:      pulumi.String(aliZones.Zones[k].Id),
			VswitchName: pulumi.String("mo-switch-" + strconv.Itoa(k)),
		}, nil)
		if err != nil {
			return nil
		}

		switchGroup = append(switchGroup, dSwitch)
	}

	return nil
}
