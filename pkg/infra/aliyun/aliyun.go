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
	"github.com/pulumi/pulumi-alicloud/sdk/v3/go/alicloud"
	"github.com/pulumi/pulumi-alicloud/sdk/v3/go/alicloud/cs"
	"github.com/pulumi/pulumi-alicloud/sdk/v3/go/alicloud/ecs"
	"github.com/pulumi/pulumi-alicloud/sdk/v3/go/alicloud/ess"
	"github.com/pulumi/pulumi-alicloud/sdk/v3/go/alicloud/vpc"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func AliyunCSDeploy(ctx *pulumi.Context, cfg *config.Config) error {

	aliZones, err := alicloud.GetZones(ctx, &alicloud.GetZonesArgs{
		AvailableResourceCreation: pulumi.StringRef("VSwitch"),
	})
	if err != nil {
		return err
	}

	aliNetwork, err := vpc.NewNetwork(ctx, "mo-vpc", &vpc.NetworkArgs{
		CidrBlock: pulumi.String("10.0.0.0/16"),
	}, nil)
	if err != nil {
		return err
	}

	aliSecurityGroup, err := ecs.NewSecurityGroup(ctx, "mo-securitygroup", &ecs.SecurityGroupArgs{
		VpcId:       aliNetwork.ID(),
		Description: pulumi.String("mo-securitygroup"),
	}, pulumi.DependsOn([]pulumi.Resource{aliNetwork}))
	if err != nil {
		return err
	}

	aliInstanceType, err := ecs.NewInstance(ctx, "mo-instance-type", &ecs.InstanceArgs{
		InstanceName:     pulumi.String("mo-instance-type"),
		InstanceType:     pulumi.String("ecs.n4.small"),
		AvailabilityZone: pulumi.String(aliZones.Zones[0].Id),
		SecurityGroups: pulumi.StringArray{
			aliSecurityGroup.ID(),
		},
		InternetChargeType: pulumi.String("PayByBandwidth"),
		Tags: pulumi.StringMap{
			"Name": pulumi.String("mo-instance-type"),
		},
	}, nil)
	if err != nil {
		return err
	}

	aliDisk, err := ecs.NewDisk(ctx, "mo-disk", &ecs.DiskArgs{
		AvailabilityZone: pulumi.String(aliZones.Zones[0].Id),
		Size:             pulumi.Int(50),
	}, pulumi.DependsOn([]pulumi.Resource{aliNetwork}))
	if err != nil {
		return err
	}

	_, err = ecs.NewDiskAttachment(ctx, "mo-instance-disk", &ecs.DiskAttachmentArgs{
		DiskId:     aliDisk.ID(),
		InstanceId: aliInstanceType.ID(),
	}, pulumi.DependsOn([]pulumi.Resource{aliInstanceType, aliDisk}))
	if err != nil {
		return err
	}

	aliScalingGroup, err := ess.NewScalingGroup(ctx, "mo-scalingGroup", &ess.ScalingGroupArgs{
		ScalingGroupName: pulumi.String("mo-scalinggroup"),
		MinSize:          pulumi.Int(1),
		MaxSize:          pulumi.Int(5),
		RemovalPolicies: pulumi.StringArray{
			pulumi.String("OldestInstance"),
			pulumi.String("NewestInstance"),
		},
	}, nil)
	if err != nil {
		return err
	}

	aliScalingConfiguration, err := ess.NewScalingConfiguration(ctx, "defaultScalingConfiguration", &ess.ScalingConfigurationArgs{
		SecurityGroupId:    aliSecurityGroup.ID(),
		ScalingGroupId:     aliScalingGroup.ID(),
		InstanceType:       aliInstanceType.ID(),
		InternetChargeType: pulumi.String("PayByTraffic"),
		ForceDelete:        pulumi.Bool(true),
		Enable:             pulumi.Bool(true),
		Active:             pulumi.Bool(true),
	}, pulumi.DependsOn([]pulumi.Resource{aliScalingGroup}))
	if err != nil {
		return err
	}

	aliManagedKubernetesCluster, err := cs.NewManagedKubernetes(ctx, "mo-managed-kubernetes-cluster", &cs.ManagedKubernetesArgs{}, nil)
	if err != nil {
		return err
	}

	_, err = cs.NewKubernetesAutoscaler(ctx, "mo-kubernetes-autoscaler", &cs.KubernetesAutoscalerArgs{
		ClusterId: aliManagedKubernetesCluster.ID(),
		Nodepools: cs.KubernetesAutoscalerNodepoolArray{
			&cs.KubernetesAutoscalerNodepoolArgs{
				Id:     aliScalingGroup.ID(),
				Labels: pulumi.String("app=mo"),
			},
		},
	}, pulumi.DependsOn([]pulumi.Resource{aliScalingConfiguration, aliScalingGroup}))
	if err != nil {
		return err
	}

	return nil
}
