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

package aws

import (
	"fmt"
	"strconv"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/eks"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func EKSDeploy(ctx *pulumi.Context) error {
	//	conf := config.New(ctx, "")
	//	ec2Size := conf.Get("ec2Size")

	vpc, err := ec2.NewVpc(ctx, "mo-pulumi-vpc", &ec2.VpcArgs{
		CidrBlock:          pulumi.String("10.0.0.0/16"),
		EnableDnsHostnames: pulumi.Bool(true),
		EnableDnsSupport:   pulumi.Bool(true),
		Tags: pulumi.StringMap{
			"Name": pulumi.String("mo-pulumi-vpc"),
		},
	})
	if err != nil {
		return err
	}
	ctx.Export("VPC_ID", vpc.ID())

	azState := "available"
	zoneList, err := aws.GetAvailabilityZones(ctx, &aws.GetAvailabilityZonesArgs{
		State: &azState,
	})
	if err != nil {
		return err
	}

	zoneNumber := 3

	var subnets []*ec2.Subnet

	for i := 0; i < zoneNumber; i++ {
		subnet, err := ec2.NewSubnet(ctx, "mo-pulumi-subnet-"+strconv.Itoa(i), &ec2.SubnetArgs{
			AvailabilityZone: pulumi.String(zoneList.Names[i]),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("mo-pulumi-subnet" + strconv.Itoa(i)),
			},
			VpcId:               vpc.ID(),
			CidrBlock:           pulumi.String("10.0." + strconv.Itoa(i) + ".0/24"),
			MapPublicIpOnLaunch: pulumi.Bool(true),
		})

		if err != nil {
			return err
		}

		subnets = append(subnets, subnet)

	}

	igw, err := ec2.NewInternetGateway(ctx, "mo-pulumi-gw", &ec2.InternetGatewayArgs{
		VpcId: vpc.ID(),
	})
	if err != nil {
		return err
	}

	_, err = ec2.NewDefaultRouteTable(ctx, "mo-pulumi-routetable", &ec2.DefaultRouteTableArgs{
		DefaultRouteTableId: vpc.DefaultRouteTableId,
		Routes: ec2.DefaultRouteTableRouteArray{
			ec2.DefaultRouteTableRouteInput(&ec2.DefaultRouteTableRouteArgs{
				CidrBlock: pulumi.String("0.0.0.0/0"),
				GatewayId: igw.ID(),
			}),
		},
	})
	if err != nil {
		return nil
	}

	eksRole, err := iam.NewRole(ctx, "eks-iam-eksRole", &iam.RoleArgs{
		AssumeRolePolicy: pulumi.String(`{
		    "Version": "2008-10-17",
		    "Statement": [{
		        "Sid": "",
		        "Effect": "Allow",
		        "Principal": {
		            "Service": "eks.amazonaws.com"
		        },
		        "Action": "sts:AssumeRole"
		    }]
		}`),
	})

	if err != nil {
		return err
	}

	eksPolicies := []string{
		"arn:aws-cn:iam::aws:policy/AmazonEKSServicePolicy",
		"arn:aws-cn:iam::aws:policy/AmazonEKSClusterPolicy",
	}
	for i, eksPolicy := range eksPolicies {
		_, err := iam.NewRolePolicyAttachment(ctx, fmt.Sprintf("rpa-%d", i), &iam.RolePolicyAttachmentArgs{
			PolicyArn: pulumi.String(eksPolicy),
			Role:      eksRole.Name,
		})
		if err != nil {
			return err
		}
	}

	// Create the EC2 NodeGroup Role
	nodeGroupRole, err := iam.NewRole(ctx, "nodegroup-iam-role", &iam.RoleArgs{
		AssumeRolePolicy: pulumi.String(`{
		    "Version": "2012-10-17",
		    "Statement": [{
		        "Sid": "",
		        "Effect": "Allow",
		        "Principal": {
		            "Service": "ec2.amazonaws.com.cn"
		        },
		        "Action": "sts:AssumeRole"
		    }]
		}`),
	})
	if err != nil {
		return err
	}

	nodeGroupPolicies := []string{
		"arn:aws-cn:iam::aws:policy/AmazonEKSWorkerNodePolicy",
		"arn:aws-cn:iam::aws:policy/AmazonEKS_CNI_Policy",
		"arn:aws-cn:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
	}
	for i, nodeGroupPolicy := range nodeGroupPolicies {
		_, err := iam.NewRolePolicyAttachment(ctx, fmt.Sprintf("ngpa-%d", i), &iam.RolePolicyAttachmentArgs{
			Role:      nodeGroupRole.Name,
			PolicyArn: pulumi.String(nodeGroupPolicy),
		})
		if err != nil {
			return err
		}
	}

	// Create a Security Group that we can use to actually connect to our cluster
	sg, err := ec2.NewSecurityGroup(ctx, "mo-pulumi-sg", &ec2.SecurityGroupArgs{
		Description: pulumi.String("security group for ec2 nodes"),
		Name:        pulumi.String("mo-pulumi-sg"),
		VpcId:       vpc.ID(),
		Egress: ec2.SecurityGroupEgressArray{
			ec2.SecurityGroupEgressArgs{
				Protocol:   pulumi.String("-1"),
				FromPort:   pulumi.Int(0),
				ToPort:     pulumi.Int(0),
				CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
			},
		},
		Ingress: ec2.SecurityGroupIngressArray{
			ec2.SecurityGroupIngressArgs{
				Protocol:   pulumi.String("tcp"),
				FromPort:   pulumi.Int(80),
				ToPort:     pulumi.Int(80),
				CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
			},
		},
	})
	if err != nil {
		return err
	}

	// Create EKS Cluster
	eksCluster, err := eks.NewCluster(ctx, "mo-pulumi-eks-cluster", &eks.ClusterArgs{
		RoleArn: pulumi.StringInput(eksRole.Arn),
		VpcConfig: &eks.ClusterVpcConfigArgs{
			PublicAccessCidrs: pulumi.StringArray{
				pulumi.String("0.0.0.0/0"),
			},
			SecurityGroupIds: pulumi.StringArray{
				sg.ID().ToStringOutput(),
			},
			SubnetIds: pulumi.StringArray{
				subnets[0].ID(),
				subnets[1].ID(),
				subnets[2].ID(),
			},
		},
	})
	if err != nil {
		return err
	}

	var nodeGroups []*eks.NodeGroup

	for i := 0; i < 1; i++ {
		ng, err := eks.NewNodeGroup(ctx, "mo-pulumi-ng-"+strconv.Itoa(i), &eks.NodeGroupArgs{
			ClusterName:   eksCluster.Name,
			NodeGroupName: pulumi.String("mo-pulumi-ng" + strconv.Itoa(i)),
			NodeRoleArn:   pulumi.StringInput(nodeGroupRole.Arn),
			SubnetIds: pulumi.StringArray{
				subnets[0].ID(),
				subnets[1].ID(),
			},
			ScalingConfig: eks.NodeGroupScalingConfigArgs{
				DesiredSize: pulumi.Int(1),
				MaxSize:     pulumi.Int(1),
				MinSize:     pulumi.Int(1),
			},
			DiskSize: pulumi.Int(100),
			Labels: pulumi.StringMap{
				"Name": pulumi.String("mo-pulumi-ng" + strconv.Itoa(i)),
			},
		})
		if err != nil {
			return err
		}

		_ = append(nodeGroups, ng)
	}

	ctx.Export("kubeconfig", generateKubeconfig(eksCluster.Endpoint,
		eksCluster.CertificateAuthority.Data().Elem(), eksCluster.Name))

	if err != nil {
		return err
	}

	return nil
}

//Create the KubeConfig Structure as per https://docs.aws.amazon.com/eks/latest/userguide/create-kubeconfig.html
func generateKubeconfig(clusterEndpoint pulumi.StringOutput, certData pulumi.StringOutput, clusterName pulumi.StringOutput) pulumi.StringOutput {
	return pulumi.Sprintf(`{
        "apiVersion": "v1",
        "clusters": [{
            "cluster": {
                "server": "%s",
                "certificate-authority-data": "%s"
            },
            "name": "kubernetes",
        }],
        "contexts": [{
            "context": {
                "cluster": "kubernetes",
                "user": "aws",
            },
            "name": "aws",
        }],
        "current-context": "aws",
        "kind": "Config",
        "users": [{
            "name": "aws",
            "user": {
                "exec": {
                    "apiVersion": "client.authentication.k8s.io/v1alpha1",
                    "command": "aws-iam-authenticator",
                    "args": [
                        "token",
                        "-i",
                        "%s",
                    ],
                },
            },
        }],
    }`, clusterEndpoint, certData, clusterName)
}
