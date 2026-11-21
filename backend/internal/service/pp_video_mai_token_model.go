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
	case PlatformCTMOSeedance:
		return ppVideoCTMOModelAliases(model)
	case PlatformGuangying:
		return ppVideoGuangyingModelAliases(model)
	case PlatformMaiTokenSeedance:
		return ppVideoMaiTokenModelAliases(model)
	case PlatformJijiuSeedance:
		return ppVideoJijiuModelAliases(model)
	case PlatformMiaowuSeedance:
		return ppVideoMiaowuModelAliases(model)
	case PlatformPaisioSeedance:
		return ppVideoPaisioModelAliases(model)
	case PlatformApophisSeedance:
		return ppVideoApophisModelAliases(model)
	case PlatformXSNSeedance:
		return ppVideoXSNModelAliases(model)
	case PlatformFengyueSeedance:
		return ppVideoFengyueModelAliases(model)
	case PlatformHongpeiSeedance:
		return ppVideoHongpeiModelAliases(model)
	case PlatformVshiSeedance:
		return ppVideoVshiModelAliases(model)
	case PlatformHuahuoSeedance:
		return ppVideoHuahuoModelAliases(model)
	case PlatformZexiSeedance:
		return ppVideoZexiModelAliases(model)
	case PlatformXMSeedance:
		return ppVideoXMModelAliases(model)
	case PlatformNaiwaSeedance:
		return ppVideoNaiwaModelAliases(model)
	case PlatformV6KSeedance:
		return ppVideoV6KModelAliases(model)
	default:
		return nil
	}
}

// PPVideoPublicModelName returns the public name of a video model for platforms that
// publish a different name from the one their upstream knows, and the input unchanged
// for every other platform. Group settings saved under an upstream name use it to keep
// matching after a model is renamed.
func PPVideoPublicModelName(platform string, model string) string {
	if platform == PlatformCTMOSeedance {
		if publicModel, ok := ppVideoCTMOPublicModel(model); ok {
			return publicModel
		}
	}
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
	if platform == PlatformMiaowuSeedance {
		if publicModel, ok := ppVideoMiaowuPublicModel(model); ok {
			return publicModel
		}
	}
	if platform == PlatformPaisioSeedance {
		if publicModel, ok := ppVideoPaisioPublicModel(model); ok {
			return publicModel
		}
	}
	if platform == PlatformApophisSeedance {
		if publicModel, ok := ppVideoApophisPublicModel(model); ok {
			return publicModel
		}
	}
	if platform == PlatformXSNSeedance {
		if publicModel, ok := ppVideoXSNPublicModel(model); ok {
			return publicModel
		}
	}
	if platform == PlatformFengyueSeedance {
		if publicModel, ok := ppVideoFengyuePublicModel(model); ok {
			return publicModel
		}
	}
	if platform == PlatformHongpeiSeedance {
		if publicModel, ok := ppVideoHongpeiPublicModel(model); ok {
			return publicModel
		}
	}
	if platform == PlatformVshiSeedance {
		if publicModel, ok := ppVideoVshiPublicModel(model); ok {
			return publicModel
		}
	}
	if platform == PlatformHuahuoSeedance {
		if publicModel, ok := ppVideoHuahuoPublicModel(model); ok {
			return publicModel
		}
	}
	if platform == PlatformZexiSeedance {
		if publicModel, ok := ppVideoZexiPublicModel(model); ok {
			return publicModel
		}
	}
	if platform == PlatformXMSeedance {
		if publicModel, ok := ppVideoXMPublicModel(model); ok {
			return publicModel
		}
	}
	if platform == PlatformNaiwaSeedance {
		if publicModel, ok := ppVideoNaiwaPublicModel(model); ok {
			return publicModel
		}
	}
	if platform == PlatformV6KSeedance {
		if publicModel, ok := ppVideoV6KPublicModel(model); ok {
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
