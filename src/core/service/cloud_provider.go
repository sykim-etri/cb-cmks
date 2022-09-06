package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/cloud-barista/cb-mcks/src/core/provision"
	"github.com/cloud-barista/cb-mcks/src/core/tumblebug"
)

const (
	AWSRoleControlPlane = "sykim-k8s-control-plane-role-for-ccm"
	AWSRoleWorker       = "sykim-k8s-worker-role-for-ccm"
)

func awsPrepareCCM(clusterName string, vms []tumblebug.VM, provisioner *provision.Provisioner) error {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return errors.New(fmt.Sprintf("Could not load default config: %v", err))
	}
	svc := ec2.NewFromConfig(cfg)

	at, err := newAWSTags(clusterName)
	if err != nil {
		return errors.New(fmt.Sprintf("Could not create tags: %v", err))
	}

	for _, vm := range vms {
		role := AWSRoleWorker
		_, exists := provisioner.ControlPlaneMachines[vm.Name]
		if exists {
			role = AWSRoleControlPlane
		}

		input := &ec2.AssociateIamInstanceProfileInput{
			IamInstanceProfile: &types.IamInstanceProfileSpecification{
				Name: &role,
			},
			InstanceId: &vm.CspViewVmDetail.IId.SystemId,
		}

		//var result *ec2.AssociateIamInstanceProfileOutput
		_, err = svc.AssociateIamInstanceProfile(context.TODO(), input)
		if err != nil {
			return errors.New(fmt.Sprintf("Could not associate IAM instance profile: %v", err))
		}

		//legacyTags := make(map[string]string)
		//legacyTags[TagNameKubernetesClusterLegacy] = clusterName
		err = at.createTags(svc, vm.CspViewVmDetail.IId.SystemId, ResourceLifecycleOwned, nil)
		if err != nil {
			return errors.New(fmt.Sprintf("Could not create tags for id(%s): %v", vm.CspViewVmDetail.IId.SystemId, err))
		}

		for _, sgid := range vm.CspViewVmDetail.SecurityGroupIIds {
			err = at.createTags(svc, sgid.SystemId, ResourceLifecycleOwned, nil)
			if err != nil {
				return errors.New(fmt.Sprintf("Could not create tags for id(%s): %v", sgid.SystemId, err))
			}
		}

		err = at.createTags(svc, vm.CspViewVmDetail.SubnetIID.SystemId, ResourceLifecycleOwned, nil)
		if err != nil {
			return errors.New(fmt.Sprintf("Could not create tags for id(%s): %v", vm.CspViewVmDetail.SubnetIID.SystemId, err))
		}
	}

	return nil
}
