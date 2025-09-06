package labels

import "github.com/kubeflow/trainer/v2/pkg/constants"

// IsSupportDeprecated returns true if labels indicate support=deprecated.
func IsSupportDeprecated(lbls map[string]string) bool {
	if lbls == nil {
		return false
	}
	val, ok := lbls[constants.LabelSupport]
	return ok && val == constants.SupportDeprecated
}
