package cloudclients

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"

	devopsv1 "github.com/andyzhang8/k8s-custom-controller/api/v1"
)

// UpdateAWSInstances ensures the running EC2 instances match 'desiredCount'.
func UpdateAWSInstances(
	ctx context.Context,
	config devopsv1.AWSConfigSpec,
	currentCount int,
	desiredCount int,
) error {
	// 1. Init EC2 client.
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(config.Region),
	})
	if err != nil {
		return fmt.Errorf("failed to create AWS session: %w", err)
	}
	ec2Svc := ec2.New(sess)

	// 2. Calculate how many instances to add or remove.
	diff := desiredCount - currentCount
	if diff > 0 {
		for i := 0; i < diff; i++ {
			if err := createEC2Instance(ctx, ec2Svc, config); err != nil {
				return err
			}
		}
	} else if diff < 0 {
		toDelete := -diff
		if err := deleteEC2Instances(ctx, ec2Svc, config, toDelete); err != nil {
			return err
		}
	}

	return nil
}

// createEC2Instance creates a single EC2 instance with the specified config.
func createEC2Instance(
	ctx context.Context,
	ec2Svc *ec2.EC2,
	config devopsv1.AWSConfigSpec,
) error {
	rand.Seed(time.Now().UnixNano())
	instanceName := fmt.Sprintf("myresource-%d", rand.Intn(1000000))

	runResult, err := ec2Svc.RunInstances(&ec2.RunInstancesInput{
		ImageId:      aws.String("ami-0abcdef1234567890"),
		InstanceType: aws.String(config.InstanceType),
		MinCount:     aws.Int64(1),
		MaxCount:     aws.Int64(1),
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String("instance"),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(instanceName),
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create EC2 instance: %w", err)
	}

	log.Printf("[AWS] Created EC2 instance: %s", *runResult.Instances[0].InstanceId)
	return nil
}

// deleteEC2Instances removes 'numToDelete' of your operator-managed instances.
func deleteEC2Instances(
	ctx context.Context,
	ec2Svc *ec2.EC2,
	config devopsv1.AWSConfigSpec,
	numToDelete int,
) error {
	describeResult, err := ec2Svc.DescribeInstances(&ec2.DescribeInstancesInput{})
	if err != nil {
		return fmt.Errorf("failed to describe EC2 instances: %w", err)
	}

	var candidateInstances []string
	for _, reservation := range describeResult.Reservations {
		for _, instance := range reservation.Instances {
			for _, tag := range instance.Tags {
				if *tag.Key == "Name" && len(*tag.Value) >= 11 && (*tag.Value)[:11] == "myresource-" {
					candidateInstances = append(candidateInstances, *instance.InstanceId)
				}
			}
		}
	}

	if len(candidateInstances) < numToDelete {
		numToDelete = len(candidateInstances)
	}

	toDeleteList := candidateInstances[:numToDelete]

	log.Printf("[AWS] Deleting %d instance(s): %v", numToDelete, toDeleteList)

	for _, instanceID := range toDeleteList {
		_, err := ec2Svc.TerminateInstances(&ec2.TerminateInstancesInput{
			InstanceIds: []*string{aws.String(instanceID)},
		})
		if err != nil {
			return fmt.Errorf("failed to terminate instance %s: %w", instanceID, err)
		}
		log.Printf("[AWS] Terminated EC2 instance: %s", instanceID)
	}

	return nil
}
