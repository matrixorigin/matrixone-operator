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
	"log"
	"strconv"

	"github.com/matrixorigin/matrixone-operator/pkg/infra/utils"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/eks"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/iam"
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes"
	apiextensions "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/apiextensions"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/core/v1"
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/helm/v3"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type InstanceCfg struct {
	Types    []string
	diskSize int
}

func EKSDeploy(ctx *pulumi.Context, cfg *config.Config) error {
	var icfg InstanceCfg
	zoneNumber := cfg.GetInt("zoneNumber")
	installOp := cfg.GetBool("installOp")
	publicAccessCidrs := cfg.GetSecret("publicAccessCidrs")
	installMOCluster := cfg.GetBool("installMOCluster")
	rcn := cfg.GetBool("regionCN")

	cfg.RequireObject("instance", &icfg)

	// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-vpc.html
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

	// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html
	azState := "available"
	zoneList, err := aws.GetAvailabilityZones(ctx, &aws.GetAvailabilityZonesArgs{
		State: &azState,
	})
	if err != nil {
		return err
	}

	if zoneNumber == 0 {
		zoneNumber = len(zoneList.Names)
	} else if zoneNumber <= 0 {
		log.Fatal("zoneNumber >= 0 !!! ")
	}

	// https://docs.aws.amazon.com/vpc/latest/userguide/configure-subnets.html
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

	// https://docs.aws.amazon.com/vpc/latest/userguide/VPC_Route_Tables.html
	_, err = ec2.NewDefaultRouteTable(ctx, "mo-pulumi-routetable", &ec2.DefaultRouteTableArgs{
		DefaultRouteTableId: vpc.DefaultRouteTableId,
		Routes: ec2.DefaultRouteTableRouteArray{
			ec2.DefaultRouteTableRouteInput(&ec2.DefaultRouteTableRouteArgs{
				CidrBlock: pulumi.String("0.0.0.0/0"),
				GatewayId: igw.ID(),
			}),
		},
	}, pulumi.DependsOn([]pulumi.Resource{vpc, igw, subnets[zoneNumber-1]}))
	if err != nil {
		return nil
	}

	// https://docs.aws.amazon.com/eks/latest/userguide/service_IAM_role.html
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
		"arn:aws:iam::aws:policy/AmazonEKSServicePolicy",
		"arn:aws:iam::aws:policy/AmazonEKSClusterPolicy",
	}
	nodeGroupPolicies := []string{
		"arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
		"arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
		"arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
	}
	assumeRolePolicy := pulumi.String(`{
		    "Version": "2012-10-17",
		    "Statement": [{
		        "Sid": "",
		        "Effect": "Allow",
		        "Principal": {
		            "Service": "ec2.amazonaws.com"
		        },
		        "Action": "sts:AssumeRole"
		    }]
		}`)

	if rcn {
		eksPolicies = []string{
			"arn:aws-cn:iam::aws:policy/AmazonEKSServicePolicy",
			"arn:aws-cn:iam::aws:policy/AmazonEKSClusterPolicy",
		}

		nodeGroupPolicies = []string{
			"arn:aws-cn:iam::aws:policy/AmazonEKSWorkerNodePolicy",
			"arn:aws-cn:iam::aws:policy/AmazonEKS_CNI_Policy",
			"arn:aws-cn:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
		}

		assumeRolePolicy = pulumi.String(`{
		    "Version": "2012-10-17",
		    "Statement": [{
		        "Sid": "",
		        "Effect": "Allow",
		        "Principal": {
		            "Service": "ec2.amazonaws.com.cn"
		        },
		        "Action": "sts:AssumeRole"
		    }]
		}`)
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

	nodeGroupRole, err := iam.NewRole(ctx, "nodegroup-iam-role", &iam.RoleArgs{
		AssumeRolePolicy: assumeRolePolicy,
	})
	if err != nil {
		return err
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
	// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/security-group-rules-reference.html
	sg, err := ec2.NewSecurityGroup(ctx, "mo-pulumi-sg", &ec2.SecurityGroupArgs{
		Description: pulumi.String("security group for ec2 nodes"),
		Name:        pulumi.String("mo-pulumi-sg"),
		VpcId:       vpc.ID(),
		Egress: ec2.SecurityGroupEgressArray{
			ec2.SecurityGroupEgressArgs{
				Protocol:   pulumi.String("-1"),
				FromPort:   pulumi.Int(0),
				ToPort:     pulumi.Int(0),
				CidrBlocks: pulumi.StringArray{pulumi.String("10.10.0.0/20")},
			},
		},
		Ingress: ec2.SecurityGroupIngressArray{
			ec2.SecurityGroupIngressArgs{
				Protocol:   pulumi.String("tcp"),
				FromPort:   pulumi.Int(80),
				ToPort:     pulumi.Int(80),
				CidrBlocks: pulumi.StringArray{pulumi.String("20.20.0.0/20")},
			},
		},
	}, pulumi.DependsOn([]pulumi.Resource{vpc, subnets[zoneNumber-1]}))
	if err != nil {
		return err
	}

	// Create EKS Cluster
	eksCluster, err := eks.NewCluster(ctx, "mo-pulumi-eks-cluster", &eks.ClusterArgs{
		RoleArn: pulumi.StringInput(eksRole.Arn),
		VpcConfig: &eks.ClusterVpcConfigArgs{
			PublicAccessCidrs: pulumi.StringArray{
				publicAccessCidrs,
			},
			SecurityGroupIds: pulumi.StringArray{
				sg.ID().ToStringOutput(),
			},
			SubnetIds: toSubnetsArray(subnets),
		},
	})
	if err != nil {
		return err
	}

	var nodeGroups []*eks.NodeGroup

	for i := 0; i < zoneNumber; i++ {
		ng, err := eks.NewNodeGroup(ctx, "mo-pulumi-ng-"+strconv.Itoa(i), &eks.NodeGroupArgs{
			ClusterName:   eksCluster.Name,
			NodeGroupName: pulumi.String("mo-pulumi-ng" + strconv.Itoa(i)),
			NodeRoleArn:   pulumi.StringInput(nodeGroupRole.Arn),
			SubnetIds:     toSubnetsArray(subnets),
			InstanceTypes: utils.ToPulumiStringArray(icfg.Types),
			ScalingConfig: eks.NodeGroupScalingConfigArgs{
				DesiredSize: pulumi.Int(1),
				MaxSize:     pulumi.Int(1),
				MinSize:     pulumi.Int(1),
			},
			DiskSize: pulumi.Int(icfg.diskSize),
			Labels: pulumi.StringMap{
				"Name": pulumi.String("mo-pulumi-ng" + strconv.Itoa(i)),
			},
		}, pulumi.DependsOn([]pulumi.Resource{eksCluster, subnets[zoneNumber-1]}))
		if err != nil {
			return err
		}

		nodeGroups = append(nodeGroups, ng)
	}

	ctx.Export("kubeconfig", generateKubeconfig(eksCluster.Endpoint,
		eksCluster.CertificateAuthority.Data().Elem(), eksCluster.Name))

	k8sProvider, err := kubernetes.NewProvider(ctx, "k8s", &kubernetes.ProviderArgs{
		Kubeconfig: generateKubeconfig(eksCluster.Endpoint, eksCluster.CertificateAuthority.Data().Elem(), eksCluster.Name),
	}, pulumi.DependsOn([]pulumi.Resource{nodeGroups[0]}))
	if err != nil {
		return err
	}

	if installMOCluster || installOp {
		opNS, err := createNS(ctx, k8sProvider, "matrixone-operator")
		if err != nil {
			return err
		}

		_, err = helm.NewRelease(ctx, "operator-helm", &helm.ReleaseArgs{
			Chart: pulumi.String("matrixone-operator"),
			RepositoryOpts: helm.RepositoryOptsArgs{
				Repo: pulumi.String("https://matrixorigin.github.io/matrixone-operator"),
			},
			Version:   pulumi.String("0.1.0"),
			Namespace: opNS.Metadata.Name(),
			SkipAwait: pulumi.BoolPtr(true),
		}, pulumi.Provider(k8sProvider), pulumi.DependsOn([]pulumi.Resource{opNS}))
		if err != nil {
			return err
		}

		if installMOCluster && !installOp {
			moNS, err := createNS(ctx, k8sProvider, "matrixone")
			if err != nil {
				return err
			}

			_, err = apiextensions.NewCustomResource(ctx, "mo-cluster", &apiextensions.CustomResourceArgs{
				ApiVersion: pulumi.String("matrixone.matrixorigin.cn/v1alpha1"),
				Kind:       pulumi.String("MatrixoneCluster"),
				Metadata: &metav1.ObjectMetaArgs{
					Name:      pulumi.String("mo"),
					Namespace: moNS.Metadata.Name(),
				},
				OtherFields: kubernetes.UntypedArgs{
					"spec": map[string]interface{}{
						"image":           pulumi.String("matrixorigin/matrixone:0.4.0"),
						"imagePullPolicy": pulumi.String("Always"),
						"replicas":        pulumi.Int(1),
						"requests": pulumi.Map{
							"cpu": pulumi.String("200m"),
						},
						"podName": pulumi.Map{
							"name": pulumi.String("POD_NAME"),
							"valueFrom": pulumi.Map{
								"fieldRef": pulumi.Map{
									"fieldPath": pulumi.String("metadata.name"),
								},
							},
						},
						"logVolumeCap":  pulumi.String("10Gi"),
						"dataVolumeCap": pulumi.String("10Gi"),
					},
				},
			}, pulumi.Provider(k8sProvider), pulumi.DependsOn([]pulumi.Resource{moNS}))
			if err != nil {
				return err
			}
		}

	}

	return nil
}

func toSubnetsArray(az []*ec2.Subnet) pulumi.StringArrayInput {
	var res []pulumi.StringInput

	for _, v := range az {
		res = append(res, v.ID().ToStringOutput())
	}

	return pulumi.StringArray(res)
}

func createNS(ctx *pulumi.Context, provider *kubernetes.Provider, ns string) (*corev1.Namespace, error) {
	n, err := corev1.NewNamespace(ctx, ns, &corev1.NamespaceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String(ns),
		},
	}, pulumi.Provider(provider))
	if err != nil {
		return n, err
	}

	return n, nil
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
                    "apiVersion": "client.authentication.k8s.io/v1beta1",
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
