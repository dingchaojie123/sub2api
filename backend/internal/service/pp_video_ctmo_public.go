package service

import "strings"

// ctmoSeedancePublicNames maps the public name a client sees to the upstream id for the CTMO (浙创)
// models whose upstream id does not say "seedance". Clients that pick video models by name
// (looking for "seedance") would otherwise not list such a model. The same approach as the XM,
// fengyue and hongpei platforms: requests may use either name, the upstream only ever receives the
// upstream id, and the model list, the task record and the response show the public name. Channel
// prices are read under the public name first, then the upstream id.
//
// Every CTMO model has "seedance" in its id today, so the table is empty (kaopu-sd-2.0-mini, the one
// model that needed it, was added on 2026-11-19 and deleted on 2026-11-20). To alias a model whose id
// does not say "seedance", add {Public: "Seedance...", Upstream: "<id>"} here and the model to
// ctmoSeedanceSupportedModels; everything else (requests, model list, task record, pricing, account
// whitelist) is already wired to this table.
var ctmoSeedancePublicNames = []struct{ Public, Upstream string }{}

// ppVideoCTMOUpstreamModel resolves either accepted name of an aliased CTMO model to its upstream id.
func ppVideoCTMOUpstreamModel(model string) (string, bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		return "", false
	}
	for _, entry := range ctmoSeedancePublicNames {
		if strings.EqualFold(entry.Public, model) || strings.EqualFold(entry.Upstream, model) {
			return entry.Upstream, true
		}
	}
	return "", false
}

// ppVideoCTMOPublicModel resolves either accepted name of an aliased CTMO model to its public name.
// A model without a separate public name is not an alias: it reports false and keeps its own name.
func ppVideoCTMOPublicModel(model string) (string, bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		return "", false
	}
	for _, entry := range ctmoSeedancePublicNames {
		if strings.EqualFold(entry.Public, model) || strings.EqualFold(entry.Upstream, model) {
			return entry.Public, true
		}
	}
	return "", false
}

// ppVideoCTMOModelAliases lists every name an aliased CTMO model answers to, public name first.
func ppVideoCTMOModelAliases(model string) []string {
	upstream, ok := ppVideoCTMOUpstreamModel(model)
	if !ok {
		return nil
	}
	public, _ := ppVideoCTMOPublicModel(model)
	return []string{public, upstream}
}

// ppVideoCTMOPublicModels maps a list of model names to the public names of the aliased models in it
// (without duplicates); every other name is kept as it is.
func ppVideoCTMOPublicModels(models []string) []string {
	result := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		name := model
		if public, ok := ppVideoCTMOPublicModel(model); ok {
			name = public
		}
		key := strings.ToLower(strings.TrimSpace(name))
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, name)
	}
	return result
}
