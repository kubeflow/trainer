/*
Copyright 2025 The Kubeflow Authors.

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

package flux

import (
	"context"
	"crypto/ecdh"
	"crypto/sha256"
	"fmt"
	"maps"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	jobsetapply "sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"

	trainer "github.com/kubeflow/trainer/v2/pkg/apis/trainer/v1alpha1"
	"github.com/kubeflow/trainer/v2/pkg/apply"
	"github.com/kubeflow/trainer/v2/pkg/constants"
	"github.com/kubeflow/trainer/v2/pkg/runtime"
	"github.com/kubeflow/trainer/v2/pkg/runtime/framework"
)

// We can customize not easily exposed MiniCluster attributes with envars
var (
	brokerDefaults = map[string]string{

		// the flux view image is the base OS / version for the view to install flux
		// ghcr.io/converged-computing/flux-view-rocky:arm-9
		// ghcr.io/converged-computing/flux-view-rocky:arm-8
		// ghcr.io/converged-computing/flux-view-rocky:tag-9
		// ghcr.io/converged-computing/flux-view-rocky:tag-8
		// ghcr.io/converged-computing/flux-view-ubuntu:tag-noble
		// ghcr.io/converged-computing/flux-view-ubuntu:tag-jammy
		// ghcr.io/converged-computing/flux-view-ubuntu:tag-focal
		// ghcr.io/converged-computing/flux-view-ubuntu:arm-jammy
		// ghcr.io/converged-computing/flux-view-ubuntu:arm-focal
		// We use an ubuntu (more recent) default since it is common
		"FLUX_VIEW_IMAGE":     "ghcr.io/converged-computing/flux-view-ubuntu:tag-jammy",
		"FLUX_NETWORK_DEVICE": "eth0",
		"FLUX_QUEUE_POLICY":   "fcfs",
		// Extra flux or broker options can be added as needed.
	}

	// Persist througout, and can be inspected / use by other controllers
	AnnotationOriginalCommand = "flux.kubeflow.org/original-command"
	AnnotationViewImage       = "flux.kubeflow.org/view-image"
	AnnotationQueuePolicy     = "flux.kubeflow.org/queue-policy"
)

var _ framework.CustomValidationPlugin = (*Flux)(nil)
var _ framework.ComponentBuilderPlugin = (*Flux)(nil)
var _ framework.EnforceMLPolicyPlugin = (*Flux)(nil)
var _ framework.WatchExtensionPlugin = (*Flux)(nil)

const Name = "Flux"

type Flux struct {
	client client.Client
	scheme *apiruntime.Scheme
}

func New(_ context.Context, client client.Client, _ client.FieldIndexer) (framework.Plugin, error) {
	return &Flux{
		client: client,
		scheme: client.Scheme(),
	}, nil
}

func (f *Flux) Name() string {
	return Name
}

func (f *Flux) Validate(_ context.Context, runtimeInfo *runtime.Info, _, newJobObj *trainer.TrainJob) (admission.Warnings, field.ErrorList) {
	var allErrs field.ErrorList
	if runtimeInfo == nil || runtimeInfo.RuntimePolicy.FluxPolicySource == nil {
		return nil, allErrs
	}

	fluxPolicy := runtimeInfo.RuntimePolicy.FluxPolicySource

	// We require at least 1 proc per node. Zero or fewer does not make sense.
	if fluxPolicy.NumProcPerNode != nil && *fluxPolicy.NumProcPerNode < 1 {
		numProcPerNodePath := field.NewPath("spec").Child("trainer").Child("numProcPerNode")
		allErrs = append(allErrs, field.Invalid(numProcPerNodePath, *fluxPolicy.NumProcPerNode, "must be greater than or equal to 1 for Flux TrainJob"))
	}

	// Iterate through Trainer's internal PodSet abstraction
	for _, ps := range runtimeInfo.TemplateSpec.PodSets {
		if ps.Name == constants.Node {
			for _, ic := range ps.InitContainers {
				if ic.Name == "flux-installer" {
					allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "trainer", "initContainers"), ic.Name, "InitContainer 'flux-installer' is reserved"))
				}
			}
		}
	}
	return nil, allErrs
}

// EnforceMLPolicy updates the JobSet
func (f *Flux) EnforceMLPolicy(info *runtime.Info, trainJob *trainer.TrainJob) error {
	if info == nil || info.RuntimePolicy.MLPolicySource == nil || info.RuntimePolicy.MLPolicySource.Flux == nil {
		fmt.Println("FluxPolicySource is nil.")
		return nil
	}

	settings := f.brokerSettingsFromEnvironment(trainJob, info)
	configMapName := fmt.Sprintf("%s-flux-entrypoint", trainJob.Name)
	curveSecretName := fmt.Sprintf("%s-flux-curve", trainJob.Name)
	sharedVolumes := getViewVolumes(configMapName)

	if trainJob.Annotations == nil {
		trainJob.Annotations = make(map[string]string)
	}

	// Capture and set the values as annotations
	// This effectively "saves" the state onto the Kubernetes resource itself
	originalCmd := getOriginalCommand(trainJob, info)
	trainJob.Annotations[AnnotationOriginalCommand] = originalCmd
	trainJob.Annotations[AnnotationViewImage] = settings["FLUX_VIEW_IMAGE"]

	// Update the command here so we wrap the original command saved earlier
	trainJob.Spec.Trainer.Command = []string{"/bin/bash", "/etc/flux-config/entrypoint.sh"}

	// Define the Init Container. This has a spack view with flux pre-built, and we add to an emptyDir
	// with configuration that is then accessible to the application. The OS/version should match.
	// For VolumeMounts, you can still use corev1ac because runtime.Container
	// methods accept the corev1ac types for nested fields
	fluxInstaller := corev1ac.Container().
		WithName("flux-installer").
		WithImage(settings["FLUX_VIEW_IMAGE"]).
		WithCommand([]string{"/bin/bash", "/etc/flux-config/init.sh"}...).
		WithVolumeMounts(
			corev1ac.VolumeMount().
				WithName("flux-install").
				WithMountPath("/mnt/flux"),
			corev1ac.VolumeMount().
				WithName(configMapName).
				WithMountPath("/etc/flux-config").
				WithReadOnly(true),
		)

	// Making changes directly to the PodSet allows them to persist
	jobSetSpec, ok := runtime.TemplateSpecApply[jobsetapply.JobSetSpecApplyConfiguration](info)
	if !ok {
		return nil
	}

	// Update the PodSets (Abstractions for the ReplicatedJobs)
	for psIdx, ps := range info.TemplateSpec.PodSets {
		if ps.Name != constants.Node {
			continue
		}

		// Add Volumes to the PodSet
		curveVolume := corev1ac.Volume().
			WithName("flux-curve").
			WithSecret(corev1ac.SecretVolumeSource().WithSecretName(curveSecretName).WithDefaultMode(0400))

		apply.UpsertVolumes(&info.TemplateSpec.PodSets[psIdx].Volumes, sharedVolumes...)
		apply.UpsertVolumes(&info.TemplateSpec.PodSets[psIdx].Volumes, *curveVolume)

		// Important! We have to add this to the JobSet to actually take
		jobSetSpec.ReplicatedJobs[psIdx].Template.Spec.Template.Spec.InitContainers = append(
			jobSetSpec.ReplicatedJobs[psIdx].Template.Spec.Template.Spec.InitContainers,
			*fluxInstaller,
		)

		// Update Containers in the PodSet
		for cIdx, container := range ps.Containers {
			if container.Name == constants.Node {
				apply.UpsertVolumeMounts(
					&info.TemplateSpec.PodSets[psIdx].Containers[cIdx].VolumeMounts,
					*corev1ac.VolumeMount().WithName("flux-install").WithMountPath("/mnt/flux"),
					*corev1ac.VolumeMount().WithName("spack-install").WithMountPath("/opt/software"),
					*corev1ac.VolumeMount().WithName(configMapName).WithMountPath("/etc/flux-config").WithReadOnly(true),
					*corev1ac.VolumeMount().WithName("flux-curve").WithMountPath("/curve").WithReadOnly(true),
				)
			}
		}
	}
	return nil
}

// Build creates the extra config map (configuration) and curve secret for Flux.
func (f *Flux) Build(ctx context.Context, info *runtime.Info, trainJob *trainer.TrainJob) ([]apiruntime.ApplyConfiguration, error) {

	// If the user's chosen runtime does not have the flux policy enabled, skip this plugin
	if info == nil || info.RuntimePolicy.FluxPolicySource == nil {
		return nil, nil
	}

	// Note that for Flux, we currently support a design that allows for
	// derivation of options from envars that are associated with the job.
	// We get these from the designated node container.
	settings := f.brokerSettingsFromEnvironment(trainJob, info)

	// We need a custom entrypoint to prepare the view and configure flux
	cm, err := f.buildInitScriptConfigMap(trainJob, info, settings)
	if err != nil {
		return nil, err
	}

	// Generate/Apply the Curve Secret deterministically based on trainjob id
	secretApply, err := f.buildCurveSecret(trainJob)
	if err != nil {
		return nil, err
	}

	// Return both. SSA will ensure they are created/merged correctly.
	return []apiruntime.ApplyConfiguration{cm, secretApply}, nil
}

func (f *Flux) ReconcilerBuilders() []runtime.ReconcilerBuilder {
	return []runtime.ReconcilerBuilder{
		func(b *builder.Builder, cl client.Client, cache cache.Cache) *builder.Builder {
			return b.Watches(
				&corev1.ConfigMap{},
				handler.EnqueueRequestForOwner(
					f.client.Scheme(), f.client.RESTMapper(), &trainer.TrainJob{}, handler.OnlyControllerOwner(),
				),
			)
		},
		func(b *builder.Builder, cl client.Client, cache cache.Cache) *builder.Builder {
			return b.Watches(
				&corev1.Secret{},
				handler.EnqueueRequestForOwner(
					f.client.Scheme(), f.client.RESTMapper(), &trainer.TrainJob{}, handler.OnlyControllerOwner(),
				),
			)
		},
	}
}

// brokerSettingsFromTrainJob derives Flux broker config settings from the jobspet node container environment.
func (f *Flux) brokerSettingsFromEnvironment(trainJob *trainer.TrainJob, info *runtime.Info) map[string]string {

	// All settings defaults that we support are already defined here
	settings := maps.Clone(brokerDefaults)

	if trainJob.Spec.Trainer.Env == nil {
		return settings
	}

	// Look through the envars in the runtime spec.
	// We only care about the environment defined for the main workers/nodes
	if info != nil {
		trainerContainer := info.FindContainerByPodSetAncestorContainerName(constants.AncestorTrainer, constants.Node)
		if trainerContainer != nil {
			for _, envar := range trainerContainer.Env {
				if envar.Name != nil && envar.Value != nil {
					if _, ok := settings[*envar.Name]; ok {
						settings[*envar.Name] = *envar.Value
					}
				}
			}
		}
	}

	// TrainJob (user) gets first preference
	// If the variable name matches one of our Flux settings, override it
	for _, envar := range trainJob.Spec.Trainer.Env {
		if _, ok := settings[envar.Name]; ok {
			settings[envar.Name] = envar.Value
		}
	}
	return settings
}

// getViewVolumes returns the volume apply configurations for the flux view setup
// We need everything here except the curve certificate
func getViewVolumes(configMapName string) []corev1ac.VolumeApplyConfiguration {
	spackInstallAC := corev1ac.Volume().
		WithName("spack-install").
		WithEmptyDir(corev1ac.EmptyDirVolumeSource())
	fluxVolumeAC := corev1ac.Volume().
		WithEmptyDir(corev1ac.EmptyDirVolumeSource()).
		WithName("flux-install")
	cmAC := corev1ac.Volume().
		WithName(configMapName).
		WithConfigMap(
			corev1ac.ConfigMapVolumeSource().
				WithName(configMapName).
				WithDefaultMode(0755),
		)
	return []corev1ac.VolumeApplyConfiguration{*spackInstallAC, *fluxVolumeAC, *cmAC}
}

// buildInitScriptConfigMap creates a ConfigMapApplyConfiguration to support server-side Apply
func (f *Flux) buildInitScriptConfigMap(
	trainJob *trainer.TrainJob,
	info *runtime.Info,
	settings map[string]string,
) (*corev1ac.ConfigMapApplyConfiguration, error) {

	// The entrypoint script finishes Flux setup and executes the wrapped application
	initScript := generateInitEntrypoint(trainJob, settings)
	entrypointScript := f.generateFluxEntrypoint(trainJob, info)

	// Build the ConfigMap using the Apply Configuration pattern
	configMapName := fmt.Sprintf("%s-flux-entrypoint", trainJob.Name)

	cmApply := corev1ac.ConfigMap(configMapName, trainJob.Namespace).
		WithOwnerReferences(metav1ac.OwnerReference().
			WithAPIVersion(trainer.SchemeGroupVersion.String()).
			WithKind(trainer.TrainJobKind).
			WithName(trainJob.Name).
			WithUID(trainJob.UID).
			WithController(true).
			WithBlockOwnerDeletion(true),
		).
		WithData(map[string]string{
			// entrypoint for application container
			"entrypoint.sh": entrypointScript,
			// entrypoint for init container (configuration)
			"init.sh": initScript,
		})

	return cmApply, nil
}

// generateBrokerConfig writes the entrypoint file, which prepares the install and configures Flux
func generateBrokerConfig(
	trainJob *trainer.TrainJob,
	hosts string,
	settings map[string]string,
) string {

	// Get the network device for Flux to use (or fall back to default)
	networkDevice := settings["FLUX_NETWORK_DEVICE"]
	queuePolicy := settings["FLUX_QUEUE_POLICY"]

	subdomain := trainJob.Name
	fqdn := fmt.Sprintf("%s.%s.svc.cluster.local", subdomain, trainJob.Namespace)

	// TODO: we can eventually derive network device from init container
	// These shouldn't be formatted in block
	defaultBind := "tcp://" + networkDevice + ":%p"
	defaultConnect := "tcp://%h" + fmt.Sprintf(".%s:", fqdn) + "%p"

	// The Flux broker configuration for the Flux Framework HPC cluster
	template := `[access]
allow-guest-user = true
allow-root-owner = true

# Point to resource definition generated with flux-R(1).
[resource]
path = "/mnt/flux/config/etc/flux/system/R"

[bootstrap]
curve_cert = "/mnt/flux/config/etc/curve/curve.cert"
default_port = 8050
default_bind = "%s"
default_connect = "%s"
hosts = [
{ host="%s"},
]

[archive]
dbpath = "/mnt/flux/config/var/lib/flux/job-archive.sqlite"
period = "1m"
busytimeout = "50s"

[sched-fluxion-qmanager]
queue-policy = "%s"
`
	return fmt.Sprintf(
		template,
		defaultBind,
		defaultConnect,
		hosts,
		queuePolicy,
	)
}

// getOriginalCommand derives the original Kubeflow command we need to wrap / handoff to Flux
func getOriginalCommand(trainJob *trainer.TrainJob, info *runtime.Info) string {
	var command []string
	var args []string

	// check PodSets first
	trainerContainer := info.FindContainerByPodSetAncestorContainerName(constants.AncestorTrainer, constants.Node)
	if trainerContainer != nil {
		command = trainerContainer.Command
	}

	// Override if user defined them in the top-level Trainer spec
	if trainJob.Spec.Trainer != nil {
		if trainJob.Spec.Trainer.Command != nil {
			command = trainJob.Spec.Trainer.Command
		}
		if trainJob.Spec.Trainer.Args != nil {
			args = trainJob.Spec.Trainer.Args
		}
	}

	// Combine into a single string for the shell script
	fullCommand := strings.Join(append(command, args...), " ")
	return strings.TrimSpace(fullCommand)
}

// generateFluxEntrypoint generates the flux entrypoint to prepare the view and run the job
func (f *Flux) generateFluxEntrypoint(trainJob *trainer.TrainJob, info *runtime.Info) string {
	mainHost := fmt.Sprintf("%s-%s-0-0", trainJob.Name, constants.Node)

	// Derive the original command intended by the user
	// command := getOriginalCommand(trainJob, info)

	// TODO we can set strict mode as an option
	script := `#!/bin/sh

fluxuser=$(whoami)
fluxuid=$(id -u $fluxuser)

# Ensure spack view is on the path, wherever it is mounted
viewbase="/mnt/flux"
viewroot=${viewbase}/view
configroot=${viewbase}/config
software="${viewbase}/software"
viewbin="${viewroot}/bin"
fluxpath=${viewbin}/flux

# Important to add AFTER in case software in container duplicated (e.g., Python)
export PATH=$PATH:${viewbin}

# Copy mount software to /opt/software
cp -R ${viewbase}/software/* /opt/software/

# Flux should use the Python with its install
foundroot=$(find $viewroot -maxdepth 2 -type d -path $viewroot/lib/python3\*) > /dev/null 2>&1
pythonversion=$(basename ${foundroot})
pythonversion=${viewroot}/bin/${pythonversion}
echo "Python version: $pythonversion"
echo "Python root: $foundroot"

# If we found the right python, ensure it's linked (old link does not work)
if [[ -f "${pythonversion}" ]]; then
   rm -rf $viewroot/bin/python3
   rm -rf $viewroot/bin/python
   ln -s ${pythonversion} $viewroot/lib/python  || true
   ln -s ${pythonversion} $viewroot/lib/python3 || true
fi

# Ensure we have flux's python on the path
export PYTHONPATH=${PYTHONPATH:-""}:${foundroot}/site-packages
export FLUX_RC_EXTRA=$viewroot/etc/flux/rc1.d

# Write a script to load fluxion
cat <<EOT >> /tmp/load-fluxion.sh
flux module remove sched-simple
flux module load sched-fluxion-resource
flux module load sched-fluxion-qmanager
EOT
mv /tmp/load-fluxion.sh ${viewbase}/load-fluxion.sh

# Write an easy file we can source for the environment
cat <<EOT >> /tmp/flux-view.sh
#!/bin/bash
export PATH=$PATH
export PYTHONPATH=$PYTHONPATH
export LD_LIBRARY_PATH=${LD_LIBRARY_PATH:-""}:$viewroot/lib
export fluxsocket=local://${configroot}/run/flux/local
EOT
mv /tmp/flux-view.sh ${viewbase}/flux-view.sh

# Variables we can use again
cfg="${configroot}/etc/flux/config"
command="%s"

# Copy mounted curve to expected location
curvepath=/mnt/flux/config/etc/curve/curve.cert
cp /curve/curve.cert ${curvepath}

# Remove group and other read
chmod o-r ${curvepath}
chmod g-r ${curvepath}
chown -R ${fluxuid} ${curvepath}

# Generate host resources
hosts=$(cat ${configroot}/etc/flux/system/hostlist)
flux R encode --hosts=${hosts} --local > /tmp/R
mv /tmp/R ${configroot}/etc/flux/system/R

# Put the state directory in /var/lib on shared view
export STATE_DIR=${configroot}/var/lib/flux
export FLUX_OUTPUT_DIR=/tmp/fluxout
mkdir -p ${STATE_DIR} ${FLUX_OUTPUT_DIR}

# Main host <name>-0 and the fully qualified domain name
mainHost="%s"
workdir=$(pwd)

# Make cron.d directory
mkdir -p ${configroot}/etc/flux/system/cron.d
brokerOptions="-Scron.directory=${configroot}/etc/flux/system/cron.d \
  -Stbon.fanout=256 \
  -Srundir=${configroot}/run/flux  \
  -Sstatedir=${STATE_DIR} -Slocal-uri=local://$configroot/run/flux/local \
  -Slog-stderr-level=0  \
  -Slog-stderr-mode=local"

# Run an interactive cluster, giving no command to flux start
function run_interactive_cluster() {
    echo "🌀 flux broker --config-path ${cfg} ${brokerOptions}"
    flux broker --config-path ${cfg} ${brokerOptions}
}

# Start flux with the original entrypoint
if [ $(hostname) == "${mainHost}" ]; then

  echo "Command provided is: ${command}"
  if [ "${command}" == "" ]; then
    run_interactive_cluster
  else

    # If tasks are == 0, then only define nodes
    node_spec="-n2"
    node_spec="${node_spec}"
    flags="${node_spec}  "
    echo "Flags for flux are ${flags}"
    flux start  -o --config ${cfg} ${brokerOptions} flux submit ${flags} --quiet --watch ${command}
  fi

# Block run by workers
else

    # We basically sleep/wait until the lead broker is ready
    echo "🌀 flux start  -o --config ${configroot}/etc/flux/config ${brokerOptions}"

    # We can keep trying forever, don't care if worker is successful or not
    # Unless retry count is set, in which case we stop after retries
    while true
    do
        flux start -o --config ${configroot}/etc/flux/config ${brokerOptions}
        retval=$?
        if [[ "${retval}" -eq 0 ]] || [[ "false" == "true" ]]; then
             echo "The follower worker exited cleanly. Goodbye!"
             break
        fi
        echo "Return value for follower worker is ${retval}"
        echo "😪 Sleeping 15s to try again..."
        sleep 15
    done
fi

# Marker of completion, if needed
touch $viewbase/flux-operator-complete.txt
`

	// Get original command from annotations
	command := trainJob.Annotations[AnnotationOriginalCommand]

	return fmt.Sprintf(
		script,
		command,
		mainHost,
	)
}

// generateInitEntrypoint generates the flux entrypoint to prepare flux
func generateInitEntrypoint(
	trainJob *trainer.TrainJob,
	settings map[string]string,
) string {

	// fluxRoot for the view is in /opt/view/lib
	// This must be consistent between the flux-view containers
	// github.com:converged-computing/flux-views.git
	fluxRoot := "/opt/view"
	mainHost := fmt.Sprintf("%s-0", trainJob.Name)

	// Generate hostlists. The hostname (prefix) is the trainJob Name
	// We need the initial jobset size, and container command
	size := *trainJob.Spec.Trainer.NumNodes
	hosts := generateHostlist(trainJob.Name, size)
	brokerConfig := generateBrokerConfig(trainJob, hosts, settings)
	setup := `#!/bin/sh
fluxroot=%s
mainHost=%s

# We need to "install" config assets separately. We may not have write to /opt/view.
installRoot=/mnt/flux/config
echo "Hello I am hostname $(hostname) running setup."

# Always use verbose, no reason to not here
echo "Flux install root: ${fluxroot}"
export fluxroot

# Add flux to the path (if using view)
export PATH=/opt/view/bin:$PATH

# If the view doesn't exist, ensure basic paths do
mkdir -p $fluxroot/bin

# Cron directory
mkdir -p $installRoot/etc/flux/system/cron.d
mkdir -p $installRoot/var/lib/flux

# These actions need to happen on all hosts
mkdir -p $installRoot/etc/flux/system
hosts="%s"

# Echo hosts here in case the main container needs to generate
echo "${hosts}" > ${installRoot}/etc/flux/system/hostlist

# Write the broker configuration
mkdir -p ${installRoot}/etc/flux/config
cat <<EOT >> ${installRoot}/etc/flux/config/broker.toml
%s
EOT

echo
echo "🐸 Broker Configuration"
cat ${installRoot}/etc/flux/config/broker.toml

# The rundir needs to be created first, and owned by user flux
# Along with the state directory and curve certificate
mkdir -p ${installRoot}/run/flux ${installRoot}/etc/curve

viewroot=/mnt/flux
mkdir -p $viewroot/view

# Now prepare to copy finished spack view over
echo "Moving content from /opt/view to be in shared volume at $viewroot"
# Note that /opt/view is a symlink to here
view=$(ls /opt/views/._view/)
view="/opt/views/._view/${view}"

# We have to move both of these paths - spack makes link to /opt/software
# /opt/software will need to be restored in application container
cp -R ${view}/* $viewroot/view
cp -R /opt/software $viewroot/

# This is a marker to indicate the copy is done
touch $viewroot/flux-operator-done.txt
echo "Application is done."
`
	return fmt.Sprintf(
		setup,
		fluxRoot,
		mainHost,
		hosts,
		brokerConfig,
	)
}

// generateHostlist for a specific size given a host prefix and a size
// This is a replicated job so format is different
// lammps-flux-interactive-node-0-0
func generateHostlist(prefix string, size int32) string {

	// Assume a setup without bursting / changing size.
	// We can extend this in the future to allow adding hosts
	return fmt.Sprintf("%s-%s-0-[%s]", prefix, constants.Node, generateRange(size, 0))
}

// generateRange is a shared function to generate a range string
func generateRange(size int32, start int32) string {
	var rangeString string
	if size == 1 {
		rangeString = fmt.Sprintf("%d", start)
	} else {
		rangeString = fmt.Sprintf("%d-%d", start, (start+size)-1)
	}
	return rangeString
}

func encodeZ85(data []byte) string {
	const charset = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ.-:+=^!/*?&<>()[]{}@%$#"
	if len(data)%4 != 0 {
		return ""
	}
	var res strings.Builder
	for i := 0; i < len(data); i += 4 {
		value := uint32(data[i])<<24 | uint32(data[i+1])<<16 | uint32(data[i+2])<<8 | uint32(data[i+3])

		// Encode into 5 characters (Base 85)
		res.WriteByte(charset[(value/52200625)%85])
		res.WriteByte(charset[(value/614125)%85])
		res.WriteByte(charset[(value/7225)%85])
		res.WriteByte(charset[(value/85)%85])
		res.WriteByte(charset[value%85])
	}
	return res.String()
}

// buildCurveSecret generates a cluster wide curve certificate for flux
func (f *Flux) buildCurveSecret(trainJob *trainer.TrainJob) (*corev1ac.SecretApplyConfiguration, error) {
	// Generate a deterministic Secret Key from the UID
	secretSeed := sha256.Sum256([]byte(trainJob.UID))

	// Derive the Public Key using standard X25519 (CURVE25519)
	// ZeroMQ/Flux uses X25519.
	priv, err := ecdh.X25519().NewPrivateKey(secretSeed[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create curve private key: %w", err)
	}
	pub := priv.PublicKey()

	// Encode both to Z85 (40 characters each)
	z85Secret := encodeZ85(priv.Bytes())
	z85Public := encodeZ85(pub.Bytes())

	// Follow template from flux keygen curve.cert
	curveContent := fmt.Sprintf("#  ZeroMQ CURVE Secret Certificate\n"+
		"#  Generated by Kubeflow Trainer\n\n"+
		"metadata\n"+
		"    name = \"%s\"\n"+
		"curve\n"+
		"    public-key = \"%s\"\n"+
		"    secret-key = \"%s\"\n",
		trainJob.Name, z85Public, z85Secret)

	curveSecretName := fmt.Sprintf("%s-flux-curve", trainJob.Name)

	return corev1ac.Secret(curveSecretName, trainJob.Namespace).
		WithData(map[string][]byte{
			"curve.cert": []byte(curveContent),
		}).
		WithOwnerReferences(metav1ac.OwnerReference().
			WithAPIVersion(trainer.SchemeGroupVersion.String()).
			WithKind(trainer.TrainJobKind).
			WithName(trainJob.Name).
			WithUID(trainJob.UID).
			WithController(true).
			WithBlockOwnerDeletion(true)), nil
}
