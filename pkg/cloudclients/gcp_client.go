package cloudclients

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	compute "google.golang.org/api/compute/v1"
	"google.golang.org/api/option"

	// Import CRD package to use GCPConfigSpec
	devopsv1 "github.com/andyzhang8/k8s-custom-controller/api/v1"
)

// UpdateGCPInstances ensures the running GCE instances match 'desiredCount'.
// If currentCount < desiredCount, create new instances. If currentCount > desiredCount, delete.
func UpdateGCPInstances(
	ctx context.Context,
	config devopsv1.GCPConfigSpec,
	currentCount int,
	desiredCount int,
) error {
	// 1. Init GCE client with the default app cred.
	svc, err := compute.NewService(ctx, option.WithScopes(compute.ComputeScope))
	if err != nil {
		return fmt.Errorf("failed to create compute service: %w", err)
	}

	// 2. Calculate how many instances to add or remove.
	diff := desiredCount - currentCount
	if diff > 0 {
		for i := 0; i < diff; i++ {
			if err := createGCEInstance(ctx, svc, config); err != nil {
				return err
			}
		}
	} else if diff < 0 {
		toDelete := -diff
		if err := deleteGCEInstances(ctx, svc, config, toDelete); err != nil {
			return err
		}
	}

	return nil
}

// createGCEInstance creates a single GCE instance with the specified config and waits for the operation to reach "DONE" status before returning.
func createGCEInstance(
    ctx context.Context,
    svc *compute.Service,
    config devopsv1.GCPConfigSpec,
) error {
    rand.Seed(time.Now().UnixNano())
    // generate random name for instance for now
    instanceName := fmt.Sprintf("myresource-%d", rand.Intn(1000000))

    // Build the Instance object, specifying machine type, disk image, network, etc.
    instance := &compute.Instance{
        Name:        instanceName,
        MachineType: fmt.Sprintf("zones/%s/machineTypes/%s", config.Zone, config.MachineType),
        Disks: []*compute.AttachedDisk{
            {
                AutoDelete: true,
                Boot:       true,
                Type:       "PERSISTENT",
                InitializeParams: &compute.AttachedDiskInitializeParams{
                    SourceImage: "projects/debian-cloud/global/images/family/debian-11",
                },
            },
        },
        NetworkInterfaces: []*compute.NetworkInterface{
            {
                AccessConfigs: []*compute.AccessConfig{
                    {Type: "ONE_TO_ONE_NAT"},
                },
            },
        },
    }

    log.Printf("[GCP] Creating instance: %s (machineType=%s, zone=%s)",
        instanceName, config.MachineType, config.Zone)

    // Insert the instance (asynchronous oper)
    op, err := svc.Instances.Insert(config.ProjectID, config.Zone, instance).
        Context(ctx).Do()
    if err != nil {
        return fmt.Errorf("failed to create GCE instance: %w", err)
    }

    log.Printf("[GCP] Create Operation %s - initial status: %s", op.Name, op.Status)

    // Wait for the operation to reach DONE status before returning
    if err := waitForZonalOp(ctx, svc, config.ProjectID, config.Zone, op.Name); err != nil {
        return fmt.Errorf("failed waiting for insert operation %s to complete: %w", op.Name, err)
    }

    log.Printf("[GCP] Instance %s creation is DONE", instanceName)
    return nil
}

// deleteGCEInstances removes 'numToDelete' of your operator-managed instances.
func deleteGCEInstances(
	ctx context.Context,
	svc *compute.Service,
	config devopsv1.GCPConfigSpec,
	numToDelete int,
) error {
	instanceList, err := svc.Instances.List(config.ProjectID, config.Zone).
		Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to list GCE instances: %w", err)
	}

	// Filter instances by name pattern
	var candidateInstances []string
	for _, inst := range instanceList.Items {
		if len(inst.Name) >= 11 && inst.Name[:11] == "myresource-" {
			candidateInstances = append(candidateInstances, inst.Name)
		}
	}

	if len(candidateInstances) < numToDelete {
		numToDelete = len(candidateInstances)
	}

	toDeleteList := candidateInstances[len(candidateInstances)-numToDelete:]

	log.Printf("[GCP] Deleting %d instance(s): %v", numToDelete, toDeleteList)

	for _, instanceName := range toDeleteList {
		op, err := svc.Instances.Delete(config.ProjectID, config.Zone, instanceName).
			Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed to delete instance %s: %w", instanceName, err)
		}
		log.Printf("[GCP] Delete Operation %s for instance %s - status: %s",
			op.Name, instanceName, op.Status)
	}

	return nil
}


func waitForZonalOp(
	ctx context.Context,
	svc *compute.Service,
	projectID string,
	zone string,
	operationName string,
) error {
	log.Printf("[GCP] Waiting for operation %s in zone %s to complete...", operationName, zone)

	// Poll the operation until it is "DONE"
	for {
		// Get the operation status
		op, err := svc.ZoneOperations.Get(projectID, zone, operationName).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed to get operation %s: %w", operationName, err)
		}

		// Check the status
		if op.Status == "DONE" {
			if op.Error != nil && len(op.Error.Errors) > 0 {
				return fmt.Errorf("operation %s completed with errors: %v", operationName, op.Error.Errors)
			}
			log.Printf("[GCP] Operation %s completed successfully.", operationName)
			break
		}

		// Wait before polling again
		time.Sleep(2 * time.Second)
	}

	return nil
}