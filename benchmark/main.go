package main

import (
	"fmt"
	"strconv"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/ebs"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type Config struct {
	EC2Type []string
	Volume  []VolumeConfig
	Bucket  string
	Region  string
}

type VolumeConfig struct {
	VolumeType string
	VolumeIops int
	VolumeCap  int
}

const (
	proName     string = "aws-fs-test"
	defaultPath string = "/dev/sdh"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {

		var tConfig Config
		cfg := config.New(ctx, "aws-fs-test")
		cfg.RequireObject("config", &tConfig)

		fmt.Printf("EC2 Type: %v\n Volume: %v\n", tConfig.EC2Type, tConfig.Volume)

		// Create a new security group for port 80
		group, err := ec2.NewSecurityGroup(ctx, "secgrp", &ec2.SecurityGroupArgs{
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

		// create volumes
		var volList []*ebs.Volume
		for vk := range tConfig.Volume {
			volName := proName + strconv.Itoa(vk)

			vol, err := ebs.NewVolume(ctx, volName, &ebs.VolumeArgs{
				AvailabilityZone:   pulumi.String(tConfig.Region),
				Size:               pulumi.Int(tConfig.Volume[vk].VolumeCap),
				Type:               pulumi.String(tConfig.Volume[vk].VolumeType),
				Iops:               pulumi.Int(tConfig.Volume[vk].VolumeIops),
				MultiAttachEnabled: pulumi.Bool(true),
			})
			if err != nil {
				return err
			}

			volList = append(volList, vol)
		}

		// available: /dev/sd[a-z][1-15]
		// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/device_naming.html
		var deviceNameList []string
		for k := range volList {
			deviceName := fmt.Sprintf("/dev/sdh%v", k)
			deviceNameList = append(deviceNameList, deviceName)
		}

		// create instance and attach volume
		var ec2List []*ec2.Instance
		for ek, ec2T := range tConfig.EC2Type {
			iname := proName + strconv.Itoa(ek)

			// ec2 user data scripts, http service for getting result
			userScript := fmt.Sprintf("#!/bin/bash \n echo %v > index.html ./fsyncperf --path %v \n > index.html \n  nohup python -m SimpleHTTPServer 80 &", ec2T, deviceNameList)
			instance, err := ec2.NewInstance(ctx, iname, &ec2.InstanceArgs{
				Tags:                pulumi.StringMap{"Name": pulumi.String(proName)},
				InstanceType:        pulumi.String(ec2T),
				VpcSecurityGroupIds: pulumi.StringArray{group.ID()},
				UserData:            pulumi.String(userScript),
			})
			if err != nil {
				return err
			}

			ec2List = append(ec2List, instance)

			for vk, volT := range volList {
				vname := proName + strconv.Itoa(ek) + strconv.Itoa(vk)
				_, err = ec2.NewVolumeAttachment(ctx, vname, &ec2.VolumeAttachmentArgs{
					DeviceName: pulumi.String(deviceNameList[vk]),
					VolumeId:   volT.ID(),
					InstanceId: instance.ID(),
				}, nil)
				if err != nil {
					return err
				}
			}
		}

		return nil
	})
}
