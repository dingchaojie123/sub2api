package service

import "strings"

// MaiTokenSeedancePublicModel is the model name shown to downstream clients.
// MaiTokenSeedanceModel stays the name the upstream knows and is the only value
// ever sent to it. Requests may use either name, so integrations that were built
// against the upstream name keep working; everything shown to clients (model
// list, response model, documentation, price lookup) uses the public name.
const MaiTokenSeedancePublicModel = "Seedance2.5-480全参"

// ppVideoMaiTokenPublicModel resolves either accepted name of the model to its
// public name. Matching is exact (after trimming), like the upstream's own.
func ppVideoMaiTokenPublicModel(model string) (string, bool) {
	model = strings.TrimSpace(model)
	if model == MaiTokenSeedancePublicModel || model == MaiTokenSeedanceModel {
		return MaiTokenSeedancePublicModel, true
	}
	return "", false
}

// ppVideoMaiTokenModelAliases lists every name the model answers to, public name first.
func ppVideoMaiTokenModelAliases(model string) []string {
	if _, ok := ppVideoMaiTokenPublicModel(model); !ok {
		return nil
	}
	return []string{MaiTokenSeedancePublicModel, MaiTokenSeedanceModel}
}

// ppVideoMaiTokenPublicModels maps a list of model names to the public names of
// the models that belong to this platform, without duplicates.
func ppVideoMaiTokenPublicModels(models []string) []string {
	filtered := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		publicModel, ok := ppVideoMaiTokenPublicModel(model)
		if !ok {
			continue
		}
		if _, exists := seen[publicModel]; exists {
			continue
		}
		seen[publicModel] = struct{}{}
		filtered = append(filtered, publicModel)
	}
	return filtered
}

// ppVideoPricingModelAliases lists the alternative model names a channel price may
// have been configured under for platforms that publish a different name from the
// one their upstream request carries.
func ppVideoPricingModelAliases(platform string, model string) []string {
	switch platform {
	case PlatformGuangying:
		return ppVideoGuangyingModelAliases(model)
	case PlatformMaiTokenSeedance:
		return ppVideoMaiTokenModelAliases(model)
	case PlatformJijiuSeedance:
		return ppVideoJijiuModelAliases(model)
	default:
		return nil
	}
}

// PPVideoPublicModelName returns the public name of a video model for platforms that
// publish a different name from the one their upstream knows, and the input unchanged
// for every other platform. Group settings saved under an upstream name use it to keep
// matching after a model is renamed.
func PPVideoPublicModelName(platform string, model string) string {
	if platform == PlatformMaiTokenSeedance {
		if publicModel, ok := ppVideoMaiTokenPublicModel(model); ok {
			return publicModel
		}
	}
	if platform == PlatformJijiuSeedance {
		if publicModel, ok := ppVideoJijiuPublicModel(model); ok {
			return publicModel
		}
	}
	if platform == PlatformSeedanceHub {
		// Group model lists saved under a platform's own name keep matching in the hub group.
		if line, ok := SeedanceHubLineByName(model); ok {
			return line.Name
		}
	}
	return model
}
