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
	"sigs.k8s.io/jobset/client-go/applyconfiguration/jobset/v1alpha2"

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

	// Ensure we don't have an initContainer named flux-installer
	js, ok := runtime.TemplateSpecApply[v1alpha2.JobSetSpecApplyConfiguration](runtimeInfo)
	if !ok || js == nil {
		return nil, allErrs
	}

	// We have to loop through replicated jobs -> podspecs -> init containers.
	for i := range js.ReplicatedJobs {
		rj := &js.ReplicatedJobs[i]
		if rj.Name != nil && *rj.Name == constants.Node {
			podSpec := rj.Template.Spec.Template.Spec
			for _, ic := range podSpec.InitContainers {
				if ic.Name != nil && *ic.Name == "flux-installer" {
					path := field.NewPath("spec").Child("trainer").Child("initContainers").Index(0).Child("name")
					allErrs = append(allErrs, field.Invalid(path, *ic.Name, "InitContainer 'flux-installer' found, invalid name"))
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

	js, ok := runtime.TemplateSpecApply[v1alpha2.JobSetSpecApplyConfiguration](info)
	if !ok || js == nil {
		return fmt.Errorf("failed to retrieve JobSet spec from info")
	}

	settings := f.brokerSettingsFromTrainJob(trainJob)
	configMapName := fmt.Sprintf("%s-flux-entrypoint", trainJob.Name)
	curveSecretName := fmt.Sprintf("%s-flux-curve", trainJob.Name)
	sharedVolumes := getViewVolumes(configMapName)

	// Ensure we have headless service for JobSet
	ensureJobSetNetwork(js, trainJob)

	// Define the Init Container. This has a spack view with flux pre-built, and we add to an emptyDir
	// with configuration that is then accessible to the application. The OS/version should match.
	fluxInstaller := corev1ac.Container().
		WithName("flux-installer").
		WithImage(settings["FLUX_VIEW_IMAGE"]).
		WithCommand("/bin/bash", "/etc/flux-config/init.sh").
		WithVolumeMounts(
			corev1ac.VolumeMount().WithName("flux-install").WithMountPath("/mnt/flux"),
			corev1ac.VolumeMount().WithName(configMapName).WithMountPath("/etc/flux-config").WithReadOnly(true),
		)

	for i := range js.ReplicatedJobs {
		rj := &js.ReplicatedJobs[i]
		if rj.Name != nil && *rj.Name == constants.Node {
			podSpec := rj.Template.Spec.Template.Spec

			// Add the Secret Volume itself.
			curveVolume := corev1ac.Volume().
				WithName("flux-curve").
				WithSecret(corev1ac.SecretVolumeSource().
					WithSecretName(curveSecretName).
					// Flux requires 0400 permissions
					WithDefaultMode(0400))

			apply.UpsertVolumes(&podSpec.Volumes, sharedVolumes...)
			apply.UpsertVolumes(&podSpec.Volumes, *curveVolume)

			// Check if it already exists before appending
			found := false
			for _, ic := range podSpec.InitContainers {
				if ic.Name != nil && *ic.Name == "flux-installer" {
					found = true
					break
				}
			}
			if !found {
				fmt.Println("init container was not found.")
				podSpec.InitContainers = append(podSpec.InitContainers, *fluxInstaller)
			}

			for j := range podSpec.Containers {
				container := &podSpec.Containers[j]
				if container.Name != nil && *container.Name == constants.Node {
					container.WithCommand("/bin/bash", "/etc/flux-config/entrypoint.sh")
					container.WithTTY(true).WithStdin(true)
					apply.UpsertVolumeMounts(
						&container.VolumeMounts,
						*corev1ac.VolumeMount().WithName("flux-install").WithMountPath("/mnt/flux"),
						*corev1ac.VolumeMount().WithName("spack-install").WithMountPath("/opt/software"),
						*corev1ac.VolumeMount().WithName(configMapName).WithMountPath("/etc/flux-config").WithReadOnly(true),
						*corev1ac.VolumeMount().WithName("flux-curve").WithMountPath("/curve").WithReadOnly(true),
					)
				}
			}
		}
	}
	return nil
}

// Build creates the extra config map (configuration) and curve secret for Flux.
func (f *Flux) Build(ctx context.Context, info *runtime.Info, trainJob *trainer.TrainJob) ([]apiruntime.ApplyConfiguration, error) {

	// policy defines the Flux HPC cluster setup
	// Many configuration params cannot be represented in JobSet alone.
	policy := info.RuntimePolicy.FluxPolicySource

	// Don't error, but assume this can't be applied here
	js, ok := runtime.TemplateSpecApply[v1alpha2.JobSetSpecApplyConfiguration](info)
	if !ok || js == nil {
		return nil, nil
	}

	// If the user's chosen runtime does not have the flux policy enabled, skip this plugin
	if policy == nil {
		return nil, nil
	}

	// Note that for Flux, we currently support a design that allows for
	// derivation of options from envars that are associated with the job.
	// We get these from the designated node container.
	settings := f.brokerSettingsFromTrainJob(trainJob)

	// We need a custom entrypoint to prepare the view and configure flux
	cm, err := buildInitScriptConfigMap(js, trainJob, settings)
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
func (f *Flux) brokerSettingsFromTrainJob(trainJob *trainer.TrainJob) map[string]string {
	settings := maps.Clone(brokerDefaults)

	if trainJob.Spec.Trainer.Env == nil {
		return settings
	}

	// Look through the containers in the TrainJob spec
	for _, envar := range trainJob.Spec.Trainer.Env {
		// If the variable name matches one of our Flux settings, override it
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

// ensureJobSetNetwork ensures we have a headless service.
func ensureJobSetNetwork(js *v1alpha2.JobSetSpecApplyConfiguration, trainJob *trainer.TrainJob) {
	// Get a handle to the existing Network builder, or create a new one if it's nil.
	networkConfig := js.Network
	if networkConfig == nil {
		networkConfig = v1alpha2.Network()
	}

	// Ensure EnableDNSHostnames is explicitly set to true.
	networkConfig.WithEnableDNSHostnames(true)

	// If a subdomain isn't already set, default it to the job name.
	if networkConfig.Subdomain == nil || *networkConfig.Subdomain == "" {
		networkConfig.WithSubdomain(trainJob.Name)
	}

	// Set the (potentially new or modified) network config back onto the main JobSet spec builder.
	// This is the crucial step that applies the changes.
	js.WithNetwork(networkConfig)

	// Note from vsoch: I am worried about the length of the DNS name here.
	// In practice with the JobSet / rpelicated jobs, it gets too long quickly.
}

// buildInitScriptConfigMap creates a ConfigMapApplyConfiguration to support server-side Apply
func buildInitScriptConfigMap(
	js *v1alpha2.JobSetSpecApplyConfiguration,
	trainJob *trainer.TrainJob,
	settings map[string]string,
) (*corev1ac.ConfigMapApplyConfiguration, error) {

	// The entrypoint script finishes Flux setup and executes the wrapped application
	initScript := generateInitEntrypoint(js, trainJob, settings)
	entrypointScript := generateFluxEntrypoint(trainJob)

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
	js *v1alpha2.JobSetSpecApplyConfiguration,
	trainJob *trainer.TrainJob,
	hosts string,
	settings map[string]string,
) string {

	// Get the network device for Flux to use (or fall back to default)
	networkDevice := settings["FLUX_NETWORK_DEVICE"]
	queuePolicy := settings["FLUX_QUEUE_POLICY"]

	subdomain := trainJob.Name
	if js.Network != nil && js.Network.Subdomain != nil {
		subdomain = *js.Network.Subdomain
	}
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

// generateFluxEntrypoint generates the flux entrypoint to prepare the view and run the job
func generateFluxEntrypoint(trainJob *trainer.TrainJob) string {
	mainHost := fmt.Sprintf("%s-%s-0-0", trainJob.Name, constants.Node)

	// Derive the original command intended by the user
	command := getOriginalCommand(trainJob)

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
echo "Python version: $pythonversion" > /dev/null 2>&1
echo "Python root: $foundroot" > /dev/null 2>&1

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

  echo "Command provided is: ${command}" > /dev/null 2>&1
  if [ "${command}" == "" ]; then
    run_interactive_cluster
  else

    # If tasks are == 0, then only define nodes
    node_spec="-n2"
    node_spec="${node_spec}"
    flags="${node_spec}  "
    echo "Flags for flux are ${flags}" > /dev/null 2>&1
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

	return fmt.Sprintf(
		script,
		command,
		mainHost,
	)
}

// generateInitEntrypoint generates the flux entrypoint to prepare flux
func generateInitEntrypoint(
	js *v1alpha2.JobSetSpecApplyConfiguration,
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
	brokerConfig := generateBrokerConfig(js, trainJob, hosts, settings)
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
	// TODO where does the first index 0 come from?
	// TODO can we be guaranteed the pod (and network) will always be node?
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
