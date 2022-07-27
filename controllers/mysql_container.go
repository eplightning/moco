package controllers

import (
	"fmt"
	"path/filepath"

	mocov1beta2 "github.com/cybozu-go/moco/api/v1beta2"
	"github.com/cybozu-go/moco/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	appsv1ac "k8s.io/client-go/applyconfigurations/apps/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
)

func (r *MySQLClusterReconciler) makeV1MySQLDContainer(cluster *mocov1beta2.MySQLCluster) (*corev1ac.ContainerApplyConfiguration, error) {
	var source *corev1ac.ContainerApplyConfiguration

	spec := cluster.Spec.PodTemplate.Spec.DeepCopy()
	for _, c := range spec.Containers {
		if *c.Name == constants.MysqldContainerName {
			source = &c
			break
		}
	}
	if source == nil {
		return nil, fmt.Errorf("MySQLD container not found")
	}

	source.
		WithArgs("--defaults-file="+filepath.Join(constants.MySQLConfPath, constants.MySQLConfName)).
		WithLifecycle(corev1ac.Lifecycle().
			WithPreStop(corev1ac.LifecycleHandler().
				WithExec(corev1ac.ExecAction().
					WithCommand("sleep", constants.PreStopSeconds)),
			),
		).WithPorts(
		corev1ac.ContainerPort().
			WithName(constants.MySQLPortName).
			WithContainerPort(constants.MySQLPort).
			WithProtocol(corev1.ProtocolTCP),
		corev1ac.ContainerPort().
			WithName(constants.MySQLXPortName).WithContainerPort(constants.MySQLXPort).WithProtocol(corev1.ProtocolTCP),
		corev1ac.ContainerPort().
			WithName(constants.MySQLAdminPortName).
			WithContainerPort(constants.MySQLAdminPort).
			WithProtocol(corev1.ProtocolTCP),
		corev1ac.ContainerPort().
			WithName(constants.MySQLHealthPortName).
			WithContainerPort(constants.MySQLHealthPort).
			WithProtocol(corev1.ProtocolTCP),
	)

	failureThreshold := cluster.Spec.StartupWaitSeconds / 10
	if failureThreshold < 1 {
		failureThreshold = 1
	}

	source.
		WithStartupProbe(corev1ac.Probe().
			WithHTTPGet(corev1ac.HTTPGetAction().
				WithPath("/healthz").
				WithPort(intstr.FromString(constants.MySQLHealthPortName)).
				WithScheme(corev1.URISchemeHTTP)).
			WithPeriodSeconds(10).
			WithFailureThreshold(failureThreshold)).
		WithLivenessProbe(corev1ac.Probe().
			WithHTTPGet(corev1ac.HTTPGetAction().
				WithPath("/healthz").
				WithPort(intstr.FromString(constants.MySQLHealthPortName)).
				WithScheme(corev1.URISchemeHTTP))).
		WithReadinessProbe(corev1ac.Probe().
			WithHTTPGet(corev1ac.HTTPGetAction().
				WithPath("/readyz").
				WithPort(intstr.FromString(constants.MySQLHealthPortName)).
				WithScheme(corev1.URISchemeHTTP)),
		)

	source.WithVolumeMounts(
		corev1ac.VolumeMount().
			WithName(constants.TmpVolumeName).
			WithMountPath(constants.TmpPath),
		corev1ac.VolumeMount().
			WithName(constants.RunVolumeName).
			WithMountPath(constants.RunPath),
		corev1ac.VolumeMount().
			WithName(constants.VarLogVolumeName).
			WithMountPath(constants.LogDirPath),
		corev1ac.VolumeMount().
			WithName(constants.MySQLConfVolumeName).
			WithMountPath(constants.MySQLConfPath),
		corev1ac.VolumeMount().
			WithName(constants.MySQLInitConfVolumeName).
			WithMountPath(constants.MySQLInitConfPath),
		corev1ac.VolumeMount().
			WithName(constants.MySQLConfSecretVolumeName).
			WithMountPath(constants.MyCnfSecretPath).
			WithReadOnly(true),
		corev1ac.VolumeMount().
			WithName(constants.MySQLDataVolumeName).
			WithMountPath(constants.MySQLDataPath),
	)

	updateContainerWithSecurityContext(source)

	return source, nil
}

func (r *MySQLClusterReconciler) makeV1AgentContainer(cluster *mocov1beta2.MySQLCluster) *corev1ac.ContainerApplyConfiguration {
	c := corev1ac.Container().
		WithName(constants.AgentContainerName).
		WithImage(r.AgentImage)

	c.WithArgs("--max-delay", fmt.Sprintf("%ds", cluster.Spec.MaxDelaySeconds))
	if cluster.Spec.LogRotationSchedule != "" {
		c.WithArgs("--log-rotation-schedule", cluster.Spec.LogRotationSchedule)
	}

	c.WithVolumeMounts(
		corev1ac.VolumeMount().
			WithName(constants.RunVolumeName).
			WithMountPath(constants.RunPath),
		corev1ac.VolumeMount().
			WithName(constants.VarLogVolumeName).
			WithMountPath(constants.LogDirPath),
		corev1ac.VolumeMount().
			WithName(constants.GRPCSecretVolumeName).
			WithMountPath("/grpc-cert").
			WithReadOnly(true),
	).WithEnv(
		corev1ac.EnvVar().
			WithName(constants.PodNameEnvKey).
			WithValueFrom(corev1ac.EnvVarSource().
				WithFieldRef(corev1ac.ObjectFieldSelector().
					WithAPIVersion("v1").
					WithFieldPath("metadata.name")),
			),
		corev1ac.EnvVar().
			WithName(constants.ClusterNameEnvKey).
			WithValue(cluster.Name),
	).WithEnvFrom(
		corev1ac.EnvFromSource().
			WithSecretRef(corev1ac.SecretEnvSource().
				WithName(cluster.UserSecretName())),
	).WithPorts(
		corev1ac.ContainerPort().
			WithName(constants.AgentPortName).
			WithContainerPort(constants.AgentPort).
			WithProtocol(corev1.ProtocolTCP),
		corev1ac.ContainerPort().
			WithName(constants.AgentMetricsPortName).
			WithContainerPort(constants.AgentMetricsPort).
			WithProtocol(corev1.ProtocolTCP),
	).WithResources(
		corev1ac.ResourceRequirements().
			WithRequests(corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(constants.AgentContainerCPURequest),
				corev1.ResourceMemory: resource.MustParse(constants.AgentContainerMemRequest),
			}).
			WithLimits(corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(constants.AgentContainerCPULimit),
				corev1.ResourceMemory: resource.MustParse(constants.AgentContainerMemLimit),
			}),
	)

	updateContainerWithSecurityContext(c)
	updateContainerWithOverwriteContainers(cluster, c)

	return c
}

func (r *MySQLClusterReconciler) makeV1SlowQueryLogContainer(cluster *mocov1beta2.MySQLCluster, sts *appsv1ac.StatefulSetApplyConfiguration, force bool) *corev1ac.ContainerApplyConfiguration {
	stsINotNil := (sts != nil && sts.Spec != nil && sts.Spec.Template != nil && sts.Spec.Template.Spec != nil)

	if !force && stsINotNil {
		for _, c := range sts.Spec.Template.Spec.Containers {
			if *c.Name == constants.SlowQueryLogAgentContainerName {
				return &c
			}
		}
	}

	c := corev1ac.Container().
		WithName(constants.SlowQueryLogAgentContainerName).
		WithImage(r.FluentBitImage).
		WithVolumeMounts(
			corev1ac.VolumeMount().
				WithName(constants.SlowQueryLogAgentConfigVolumeName).
				WithMountPath(constants.FluentBitConfigPath).
				WithReadOnly(true),
			corev1ac.VolumeMount().
				WithName(constants.VarLogVolumeName).
				WithMountPath(constants.LogDirPath),
		).
		WithResources(
			corev1ac.ResourceRequirements().
				WithRequests(corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(constants.SlowQueryLogAgentCPURequest),
					corev1.ResourceMemory: resource.MustParse(constants.SlowQueryLogAgentMemRequest),
				}).
				WithLimits(corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(constants.SlowQueryLogAgentCPULimit),
					corev1.ResourceMemory: resource.MustParse(constants.SlowQueryLogAgentMemLimit),
				}),
		)

	updateContainerWithSecurityContext(c)
	updateContainerWithOverwriteContainers(cluster, c)

	return c
}

func (r *MySQLClusterReconciler) makeV1ExporterContainer(cluster *mocov1beta2.MySQLCluster, collectors []string) *corev1ac.ContainerApplyConfiguration {
	c := corev1ac.Container().
		WithName(constants.ExporterContainerName).
		WithImage(r.ExporterImage).
		WithArgs("--config.my-cnf="+filepath.Join(constants.MyCnfSecretPath, constants.ExporterMyCnf)).
		WithPorts(
			corev1ac.ContainerPort().
				WithName(constants.ExporterPortName).
				WithContainerPort(constants.ExporterPort).
				WithProtocol(corev1.ProtocolTCP)).
		WithVolumeMounts(
			corev1ac.VolumeMount().
				WithName(constants.RunVolumeName).
				WithMountPath(constants.RunPath),
			corev1ac.VolumeMount().
				WithName(constants.MySQLConfSecretVolumeName).
				WithMountPath(constants.MyCnfSecretPath).
				WithReadOnly(true),
		).
		WithResources(
			corev1ac.ResourceRequirements().
				WithRequests(corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(constants.ExporterContainerCPURequest),
					corev1.ResourceMemory: resource.MustParse(constants.ExporterContainerMemRequest),
				}).
				WithLimits(corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(constants.ExporterContainerCPULimit),
					corev1.ResourceMemory: resource.MustParse(constants.ExporterContainerMemLimit),
				}),
		)

	for _, cl := range collectors {
		c.WithArgs("--collect." + cl)
	}

	updateContainerWithSecurityContext(c)
	updateContainerWithOverwriteContainers(cluster, c)

	return c
}

func (r *MySQLClusterReconciler) makeV1OptionalContainers(cluster *mocov1beta2.MySQLCluster) []*corev1ac.ContainerApplyConfiguration {
	var containers []*corev1ac.ContainerApplyConfiguration

	spec := cluster.Spec.PodTemplate.Spec.DeepCopy()
	for _, c := range spec.Containers {
		c := c

		if c.Name == nil {
			continue
		}

		updateContainerWithSecurityContext(&c)

		switch *c.Name {
		case constants.MysqldContainerName:
		case constants.AgentContainerName:
		case constants.SlowQueryLogAgentContainerName:
			if cluster.Spec.DisableSlowQueryLogContainer {
				containers = append(containers, &c)
			}
		case constants.ExporterContainerName:
			if len(cluster.Spec.Collectors) == 0 {
				containers = append(containers, &c)
			}
		default:
			containers = append(containers, &c)
		}
	}
	return containers
}

func (r *MySQLClusterReconciler) makeV1InitContainer(cluster *mocov1beta2.MySQLCluster, image string) []*corev1ac.ContainerApplyConfiguration {
	c := corev1ac.Container().
		WithName(constants.InitContainerName).
		WithImage(image).
		WithCommand(
			constants.InitCommand,
			"--data-dir="+constants.MySQLDataPath,
			"--conf-dir="+constants.MySQLInitConfPath,
			fmt.Sprintf("%d", cluster.Spec.ServerIDBase),
		).WithEnv(
		corev1ac.EnvVar().
			WithName(constants.PodNameEnvKey).
			WithValueFrom(corev1ac.EnvVarSource().
				WithFieldRef(corev1ac.ObjectFieldSelector().
					WithAPIVersion("v1").
					WithFieldPath("metadata.name")),
			),
	).WithVolumeMounts(
		corev1ac.VolumeMount().
			WithName(constants.MySQLDataVolumeName).
			WithMountPath(constants.MySQLDataPath),
		corev1ac.VolumeMount().
			WithName(constants.MySQLInitConfVolumeName).
			WithMountPath(constants.MySQLInitConfPath),
	).WithResources(
		corev1ac.ResourceRequirements().
			WithRequests(corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(constants.InitContainerCPURequest),
				corev1.ResourceMemory: resource.MustParse(constants.InitContainerMemRequest),
			}).
			WithLimits(corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(constants.InitContainerCPULimit),
				corev1.ResourceMemory: resource.MustParse(constants.InitContainerMemLimit),
			}),
	)

	updateContainerWithSecurityContext(c)
	updateContainerWithOverwriteContainers(cluster, c)

	var initContainers []*corev1ac.ContainerApplyConfiguration
	initContainers = append(initContainers, c)

	spec := cluster.Spec.PodTemplate.Spec.DeepCopy()
	for _, given := range spec.InitContainers {
		ic := given
		updateContainerWithSecurityContext(&ic)
		initContainers = append(initContainers, &ic)
	}
	return initContainers
}

func updateContainerWithSecurityContext(container *corev1ac.ContainerApplyConfiguration) {
	if container.SecurityContext == nil {
		container.WithSecurityContext(corev1ac.SecurityContext())
	}
	container.SecurityContext.
		WithRunAsUser(constants.ContainerUID).
		WithRunAsGroup(constants.ContainerGID)
}

func updateContainerWithOverwriteContainers(cluster *mocov1beta2.MySQLCluster, container *corev1ac.ContainerApplyConfiguration) {
	if len(cluster.Spec.PodTemplate.OverwriteContainers) == 0 {
		return
	}

	for _, overwrite := range cluster.Spec.PodTemplate.OverwriteContainers {
		overwrite := overwrite
		if container.Name != nil && *container.Name == overwrite.Name.String() {
			if overwrite.Resources != nil {
				container.WithResources((*corev1ac.ResourceRequirementsApplyConfiguration)(overwrite.Resources))
			}
		}
	}
}
