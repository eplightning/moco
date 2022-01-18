package v1beta2

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// JobConfig is a set of parameters for backup and restore job Pods.
type JobConfig struct {
	// ServiceAccountName specifies the ServiceAccount to run the Pod.
	// +kubebuilder:validation:MinLength=1
	ServiceAccountName string `json:"serviceAccountName"`

	// Specifies how to access an object storage bucket.
	BucketConfig BucketConfig `json:"bucketConfig"`

	// WorkVolume is the volume source for the working directory.
	// Since the backup or restore task can use a lot of bytes in the working directory,
	// You should always give a volume with enough capacity.
	//
	// The recommended volume source is a generic ephemeral volume.
	// https://kubernetes.io/docs/concepts/storage/ephemeral-volumes/#generic-ephemeral-volumes
	WorkVolume corev1.VolumeSource `json:"workVolume"`

	// Threads is the number of threads used for backup or restoration.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=4
	// +optional
	Threads int `json:"threads,omitempty"`

	// Memory is the amount of memory requested for the Pod.
	// +kubebuilder:default="4Gi"
	// +nullable
	// +optional
	Memory *resource.Quantity `json:"memory,omitempty"`

	// MaxMemory is the amount of maximum memory for the Pod.
	// +nullable
	// +optional
	MaxMemory *resource.Quantity `json:"maxMemory,omitempty"`

	// List of sources to populate environment variables in the container.
	// The keys defined within a source must be a C_IDENTIFIER. All invalid keys
	// will be reported as an event when the container is starting. When a key exists in multiple
	// sources, the value associated with the last source will take precedence.
	// Values defined by an Env with a duplicate key will take precedence.
	//
	// You can configure S3 bucket access parameters through environment variables.
	// See https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/config#EnvConfig
	//
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`

	// List of environment variables to set in the container.
	//
	// You can configure S3 bucket access parameters through environment variables.
	// See https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/config#EnvConfig
	//
	// +optional
	// +listType=map
	// +listMapKey=name
	Env []corev1.EnvVar `json:"env,omitempty"`
}

// BucketConfig is a set of parameter to access an object storage bucket.
type BucketConfig struct {
	// The name of the bucket
	// +kubebuilder:validation:MinLength=1
	BucketName string `json:"bucketName"`

	// The region of the bucket.
	// This can also be set through `AWS_REGION` environment variable.
	// +optional
	Region string `json:"region,omitempty"`

	// The API endpoint URL.  Set this for non-S3 object storages.
	// +kubebuilder:validation:Pattern="^https?://.*"
	// +optional
	EndpointURL string `json:"endpointURL,omitempty"`

	// Allows you to enable the client to use path-style addressing, i.e.,
	// https?://ENDPOINT/BUCKET/KEY. By default, a virtual-host addressing
	// is used (https?://BUCKET.ENDPOINT/KEY).
	// +optional
	UsePathStyle bool `json:"usePathStyle,omitempty"`
}
