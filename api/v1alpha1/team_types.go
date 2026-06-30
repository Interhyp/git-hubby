<<<<<<< HEAD
=======
/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
>>>>>>> tmp-original-30-06-26-04-09
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// TeamSpec defines the desired state of Team within one or more Organizations.
// Teams group organization members and can be assigned permissions to repositories.
// A Team can exist in multiple organizations simultaneously.
// See: https://docs.github.com/en/rest/teams/teams
// +kubebuilder:validation:ExactlyOneOf=idpGroup;members
type TeamSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// Name is the display name of the team in GitHub.
	// GitHub automatically generates a "slug" from this name for use in URLs and APIs.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=100
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,99}$`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type=string
	Name string `json:"name"`

	// Members is a list of GitHub usernames to add to the team.
	// This field is mutually exclusive with IDPGroup.
	// When set, team membership is managed manually through this list.
	// Members not in this list will be removed from the team.
	// +kubebuilder:validation:MaxItems=100
	Members []string `json:"members,omitempty"`

	// IDPGroup is the name of the Identity Provider group to synchronize with this team.
	// This field is mutually exclusive with Members.
	// When set, team membership is automatically synchronized from the IDP group.
	// See: https://docs.github.com/en/organizations/organizing-members-into-teams/synchronizing-a-team-with-an-identity-provider-group
	// +kubebuilder:validation:MaxLength=100
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,99}$`
	// +kubebuilder:validation:Type=string
	IDPGroup *string `json:"idpGroup,omitempty"`

	// Description provides additional information about the team's purpose.
	// This appears on the team's page in GitHub.
	// +kubebuilder:validation:MaxLength=1000
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Type=string
	Description string `json:"description,omitempty"`

	// Privacy controls the visibility of the team within the organization.
	// - "closed": The team is visible to all members of the organization, but only team members can see team discussions and manage team membership.
	// - "secret": The team is only visible to organization owners and team members.
	// See: https://docs.github.com/en/rest/teams/teams#create-a-team
	// +kubebuilder:validation:Enum=closed;secret
	// +kubebuilder:default=closed
	// +kubebuilder:validation:Optional
	Privacy string `json:"privacy,omitempty"`

	// Permission specifies the default permission granted to team members for organization repositories.
	// - "pull": Team members can pull (read) from organization repositories.
	// - "push": Team members can pull and push (read and write) to organization repositories.
	// Note: This is a legacy field. Use organization roles for more fine-grained permissions.
	// See: https://docs.github.com/en/rest/teams/teams#create-a-team
	// +kubebuilder:validation:Enum=pull;push
	// +kubebuilder:default=pull
	// +kubebuilder:validation:Optional
	Permission string `json:"permission,omitempty"`

	// NotificationSetting controls whether team members receive notifications for the team.
	// - "notifications_disabled": No one receives notifications.
	// - "notifications_enabled": Everyone receives notifications when the team is @mentioned.
	// See: https://docs.github.com/en/rest/teams/teams#create-a-team
	// +kubebuilder:validation:Enum=notifications_disabled;notifications_enabled
	// +kubebuilder:default=notifications_disabled
	// +kubebuilder:validation:Optional
	NotificationSetting string `json:"notificationSetting,omitempty"`

	// OrganizationRoles is a list of organization role names to assign to this team.
	// Organization roles define the permissions the team has within the organization.
	// If not specified, defaults to empty list.
	// Set to an empty list to remove all role assignments.
	// See: https://docs.github.com/en/rest/orgs/organization-roles
	// +kubebuilder:validation:Optional
	// +listType=set
	OrganizationRoles []string `json:"organizationRoles"`

	// OrganizationRefs is a list of Organization CRDs that this team belongs to.
	// The team will be created or updated in all referenced organizations.
	// Removing an organization from this list will delete the team from that organization
	// while preserving it in other organizations.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	OrganizationRefs []OrganizationRef `json:"organizationRefs,omitempty"`
}

// TeamStatus defines the observed state of Team.
type TeamStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the Team resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// PreviousOrganizationRefs tracks the organization references from the last successful reconciliation.
	// This allows the reconciler to detect when organizations are removed from the spec
	// and clean up teams from those organizations while preserving them in remaining organizations.
	// +optional
	PreviousOrganizationRefs []OrganizationRef `json:"previousOrganizationRefs,omitempty"`

	// Slug is the URL-friendly version of the team name as assigned by GitHub.
	// This slug is used in URLs and API calls. GitHub generates it automatically from the Name field.
	// Example: A team named "Platform Engineers" might have the slug "platform-engineers".
	Slug *string `json:"slug,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource

type Team struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Team
	// +required
	Spec TeamSpec `json:"spec"`

	// status defines the observed state of Team
	// +optional
	Status TeamStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// TeamList contains a list of Teams
type TeamList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Team `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Team{}, &TeamList{})
		return nil
	})
}
