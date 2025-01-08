package cloudclients

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"

	devopsv1 "github.com/andyzhang8/k8s-custom-controller/api/v1"
)

// UpdateAzureInstances ensures the running Azure VMs match 'desiredCount'.
func UpdateAzureInstances(
	ctx context.Context,
	config devopsv1.AzureConfigSpec,
	currentCount int,
	desiredCount int,
) error {
	// 1. Initialize Azure clients.
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return fmt.Errorf("failed to obtain Azure credential: %w", err)
	}

	vmClient, err := armcompute.NewVirtualMachinesClient(config.SubscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create VM client: %w", err)
	}

	diff := desiredCount - currentCount
	if diff > 0 {
		for i := 0; i < diff; i++ {
			if err := createAzureVM(ctx, vmClient, config); err != nil {
				return err
			}
		}
	} else if diff < 0 {
		toDelete := -diff
		if err := deleteAzureVMs(ctx, vmClient, config, toDelete); err != nil {
			return err
		}
	}

	return nil
}

// createAzureVM creates a single Azure VM with the specified config.
func createAzureVM(
	ctx context.Context,
	vmClient *armcompute.VirtualMachinesClient,
	config devopsv1.AzureConfigSpec,
) error {
	rand.Seed(time.Now().UnixNano())
	vmName := fmt.Sprintf("myresource-%d", rand.Intn(1000000))

	log.Printf("[Azure] Creating VM: %s in resource group: %s", vmName, config.ResourceGroup)

	vmParams := armcompute.VirtualMachine{
		Location: &config.Region,
		Properties: &armcompute.VirtualMachineProperties{
			HardwareProfile: &armcompute.HardwareProfile{
				VMSize: armcompute.VirtualMachineSizeTypes(config.VMSize),
			},
			StorageProfile: &armcompute.StorageProfile{
				ImageReference: &armcompute.ImageReference{
					Publisher: &config.ImagePublisher,
					Offer:     &config.ImageOffer,
					SKU:       &config.ImageSKU,
					Version:   &config.ImageVersion,
				},
			},
			OSProfile: &armcompute.OSProfile{
				ComputerName:  &vmName,
				AdminUsername: &config.AdminUsername,
				AdminPassword: &config.AdminPassword,
			},
			NetworkProfile: &armcompute.NetworkProfile{
				NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
					{
						ID: &config.NetworkInterfaceID,
					},
				},
			},
		},
	}

	pollerResp, err := vmClient.BeginCreateOrUpdate(ctx, config.ResourceGroup, vmName, vmParams, nil)
	if err != nil {
		return fmt.Errorf("failed to start VM creation: %w", err)
	}

	_, err = pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create VM: %w", err)
	}

	log.Printf("[Azure] VM %s creation completed successfully.", vmName)
	return nil
}

// Removes 'numToDelete' VMs managed by the operator.
func deleteAzureVMs(
	ctx context.Context,
	vmClient *armcompute.VirtualMachinesClient,
	config devopsv1.AzureConfigSpec,
	numToDelete int,
) error {
	// List VMs in the resource group.
	pager := vmClient.NewListPager(config.ResourceGroup, nil)
	var candidateVMs []string

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list VMs: %w", err)
		}
		for _, vm := range page.Value {
			if vm.Name != nil && len(*vm.Name) >= 11 && (*vm.Name)[:11] == "myresource-" {
				candidateVMs = append(candidateVMs, *vm.Name)
			}
		}
	}

	if len(candidateVMs) < numToDelete {
		numToDelete = len(candidateVMs)
	}

	toDeleteList := candidateVMs[:numToDelete]
	log.Printf("[Azure] Deleting %d VM(s): %v", numToDelete, toDeleteList)

	for _, vmName := range toDeleteList {
		pollerResp, err := vmClient.BeginDelete(ctx, config.ResourceGroup, vmName, nil)
		if err != nil {
			return fmt.Errorf("failed to start VM deletion for %s: %w", vmName, err)
		}
		_, err = pollerResp.PollUntilDone(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to delete VM %s: %w", vmName, err)
		}
		log.Printf("[Azure] VM %s deletion completed successfully.", vmName)
	}

	return nil
}
