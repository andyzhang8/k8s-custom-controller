package v1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GCPConfigSpec struct {
    ProjectID string `json:"projectID,omitempty"`
    Region string `json:"region,omitempty"`
    Zone string `json:"zone,omitempty"`
    MachineType string `json:"machineType,omitempty"`
    CredentialsSecretRef string `json:"credentialsSecretRef,omitempty"`
}

type AWSConfigSpec struct {
    Region             string `json:"region,omitempty"`
    InstanceType       string `json:"instanceType,omitempty"`
    AdminUsername      string `json:"adminUsername,omitempty"`
    AdminPassword      string `json:"adminPassword,omitempty"`
    NetworkInterfaceID string `json:"networkInterfaceID,omitempty"`
    SubscriptionID     string `json:"subscriptionID,omitempty"`
    ResourceGroup      string `json:"resourceGroup,omitempty"`
}

type AzureConfigSpec struct {
    Region             string `json:"region,omitempty"`
    VMSize             string `json:"vmSize,omitempty"`
    ImagePublisher     string `json:"imagePublisher,omitempty"`
    ImageOffer         string `json:"imageOffer,omitempty"`
    ImageSKU           string `json:"imageSKU,omitempty"`
    ImageVersion       string `json:"imageVersion,omitempty"`
    AdminUsername      string `json:"adminUsername,omitempty"`
    AdminPassword      string `json:"adminPassword,omitempty"`
    NetworkInterfaceID string `json:"networkInterfaceID,omitempty"`
    SubscriptionID     string `json:"subscriptionID,omitempty"`
    ResourceGroup      string `json:"resourceGroup,omitempty"`
}

// MyResourceSpec defines the desired state of MyResource.
type MyResourceSpec struct {
    DesiredCount int `json:"desiredCount,omitempty"`
    GCPConfig    *GCPConfigSpec    `json:"gcpConfig,omitempty"`
    AWSConfig    *AWSConfigSpec    `json:"awsConfig,omitempty"`
    AzureConfig  *AzureConfigSpec  `json:"azureConfig,omitempty"`
}

// MyResourceStatus defines the observed state of MyResource.
type MyResourceStatus struct {
    CurrentCount int    `json:"currentCount,omitempty"`
    Phase        string `json:"phase,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MyResource is the Schema for the MyResource API.
type MyResource struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              MyResourceSpec   `json:"spec,omitempty"`
    Status            MyResourceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MyResourceList contains a list of MyResource.
type MyResourceList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []MyResource `json:"items"`
}
