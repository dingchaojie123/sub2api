package service

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/tidwall/gjson"
)

// The Seedance aggregate group (platform seedance-hub) serves the Seedance models of several
// upstream platforms through one group and one API key. Every line has its own public name,
// of the form
//
//	Seedance{version}--{resolutions}--{model}--{billing}--{platform letter}
//
// and belongs to exactly one platform, so routing is a fixed lookup, not a search. A request
// for a line is handled like a request sent to a group of that line's platform: the `model`
// of the body is rewritten to the name the platform's own pipeline accepts (Model below), and
// everything after that (v2 translation, normalisation, account selection, forwarding,
// settlement) is the platform's existing code.
//
// Every name a line answers to (its public name, the platform's own name, and the aliases
// the platform already accepts) must be unique across all lines; the tests check it. Clients
// that already use the platform's own name keep working inside the hub group.

const (
	seedanceHubVersion20 = "2.0"
	seedanceHubVersion25 = "2.5"
)

// SeedanceHubLine is one model line of the aggregate group.
type SeedanceHubLine struct {
	// Name is the public name in the hub group (shown in the model list, the task record and
	// the response, and the name channel prices are configured under).
	Name string
	// Platform serves the line.
	Platform string
	// Model is the name the platform's own request pipeline accepts; the request's `model` is
	// rewritten to it before the platform pipeline runs.
	Model string
	// Version is "2.0" or "2.5".
	Version string
	// Billing is how the line is billed: BillingModePerRequest or BillingModeVideo (per second).
	// It decides the order of the model list; the channel price decides the real billing.
	Billing BillingMode
	// Limits are the documented limits of the line. Only lines whose platform code has no limit
	// table of its own carry them; the others report their platform's limits.
	Limits *seedanceHubLimits
}

type seedanceHubLimits struct {
	MinSeconds  int
	MaxSeconds  int
	Resolutions []string
	MaxImages   int
	MaxVideos   int
	MaxAudios   int
}

// seedanceHubLines is in the order the lines were agreed on: by platform, then by model. The
// order of the model list is derived from it by seedanceHubSortedLines.
var seedanceHubLines = []SeedanceHubLine{
	// 浙创 (Z)
	{
		Name: "Seedance2.0--720p--930--按次--Z", Platform: PlatformCTMOSeedance, Model: "seedance-2.0-930",
		Version: seedanceHubVersion20, Billing: BillingModePerRequest,
		Limits: &seedanceHubLimits{MinSeconds: 5, MaxSeconds: 15, Resolutions: []string{"720p"}, MaxImages: 9, MaxVideos: 3, MaxAudios: 0},
	},
	{
		Name: "Seedance2.0--720p--903--按次--Z", Platform: PlatformCTMOSeedance, Model: "seedance-2.0-903",
		Version: seedanceHubVersion20, Billing: BillingModePerRequest,
		Limits: &seedanceHubLimits{MinSeconds: 5, MaxSeconds: 15, Resolutions: []string{"720p"}, MaxImages: 9, MaxVideos: 0, MaxAudios: 3},
	},
	// 光影 (G)
	{Name: "Seedance2.5--720p--全参--按秒--G", Platform: PlatformGuangying, Model: "Seedance 2.5-全参720P", Version: seedanceHubVersion25, Billing: BillingModeVideo},
	{Name: "Seedance2.5--720p--限时满参--按次--G", Platform: PlatformGuangying, Model: "限时限量-Seedance 2.5 满参", Version: seedanceHubVersion25, Billing: BillingModePerRequest},
	{Name: "Seedance2.5--720p--限时满参备选--按次--G", Platform: PlatformGuangying, Model: "限时限量-Seedance 2.5 满参-备选", Version: seedanceHubVersion25, Billing: BillingModePerRequest},
	{Name: "Seedance2.5--720p--无视频--按秒--G", Platform: PlatformGuangying, Model: "Seedance 2.5-无视频", Version: seedanceHubVersion25, Billing: BillingModeVideo},
	{Name: "Seedance2.5--480p/720p--无参考视频--按秒--G", Platform: PlatformGuangying, Model: "Seedance 2.5-无参考视频 按秒计费", Version: seedanceHubVersion25, Billing: BillingModeVideo},
	{Name: "Seedance2.0--720p--标准--按次--G", Platform: PlatformGuangying, Model: "Seedance 2.0按次计费", Version: seedanceHubVersion20, Billing: BillingModePerRequest},
	{Name: "Seedance2.0--480p/720p/1080p--无参考视频--按秒--G", Platform: PlatformGuangying, Model: "Seedance 2.0-无参考视频 按秒计费", Version: seedanceHubVersion20, Billing: BillingModeVideo},
	{Name: "Seedance2.0--720p--竖屏--按次--G", Platform: PlatformGuangying, Model: "竖屏-Seedance 2.0按次计费", Version: seedanceHubVersion20, Billing: BillingModePerRequest},
	{Name: "Seedance2.0--720p--竖屏Fast--按次--G", Platform: PlatformGuangying, Model: "竖屏-Seedance 2.0 Fast 按次计费", Version: seedanceHubVersion20, Billing: BillingModePerRequest},
	{Name: "Seedance2.0--720p--竖屏Mini--按次--G", Platform: PlatformGuangying, Model: "竖屏-Seedance 2.0 Mini 按次计费", Version: seedanceHubVersion20, Billing: BillingModePerRequest},
	// 星光 (X)
	{
		Name: "Seedance2.0--720p--933--按次--X", Platform: PlatformXingAPISeedance, Model: "seedance-933",
		Version: seedanceHubVersion20, Billing: BillingModePerRequest,
		Limits: &seedanceHubLimits{MinSeconds: 4, MaxSeconds: 15, Resolutions: []string{"720p"}, MaxImages: 9, MaxVideos: 0, MaxAudios: 3},
	},
	{
		Name: "Seedance2.0--720p--933-C--按次--X", Platform: PlatformXingAPISeedance, Model: "seedance2.0-933-C",
		Version: seedanceHubVersion20, Billing: BillingModePerRequest,
		Limits: &seedanceHubLimits{MinSeconds: 4, MaxSeconds: 15, Resolutions: []string{"720p"}, MaxImages: 9, MaxVideos: 3, MaxAudios: 3},
	},
	{
		Name: "Seedance2.5--720p--V--按次--X", Platform: PlatformXingAPISeedance, Model: "seedance2.5-V",
		Version: seedanceHubVersion25, Billing: BillingModePerRequest,
		Limits: &seedanceHubLimits{MinSeconds: 5, MaxSeconds: 30, Resolutions: []string{"720p"}, MaxImages: 30, MaxVideos: 10, MaxAudios: 10},
	},
	// Mai Token (M)
	{Name: "Seedance2.5--480p--全参--按秒--M", Platform: PlatformMaiTokenSeedance, Model: MaiTokenSeedancePublicModel, Version: seedanceHubVersion25, Billing: BillingModeVideo},
	// 极九 (J)
	{Name: "Seedance2.5--1080p--全参C--按秒--J", Platform: PlatformJijiuSeedance, Model: "满血seedance-2.5全参C", Version: seedanceHubVersion25, Billing: BillingModeVideo},
	{Name: "Seedance2.5--480p/720p/1080p--全参D--按秒--J", Platform: PlatformJijiuSeedance, Model: "稳定seedance-2.5全参D", Version: seedanceHubVersion25, Billing: BillingModeVideo},
	{Name: "Seedance2.5--480p--参图参音F--按秒--J", Platform: PlatformJijiuSeedance, Model: "稳定seedance-2.5参图参音F", Version: seedanceHubVersion25, Billing: BillingModeVideo},
	{Name: "Seedance2.0--480p/720p/1080p--933线路一--按秒--J", Platform: PlatformJijiuSeedance, Model: "SD2.0满血稳定933-线路一", Version: seedanceHubVersion20, Billing: BillingModeVideo},
	// 903z is billed per request in the channel (the operations sheet of 2026-11-10), although the
	// upstream bills it per second, so the billing segment of the name says 按次.
	{Name: "Seedance2.0--720p--903z--按次--J", Platform: PlatformJijiuSeedance, Model: "SD2.0特价稳定903z", Version: seedanceHubVersion20, Billing: BillingModePerRequest},
	{Name: "Seedance2.0--720p--903mini--按次--J", Platform: PlatformJijiuSeedance, Model: "SD2.0mini特价稳定903z", Version: seedanceHubVersion20, Billing: BillingModePerRequest},
	{Name: "Seedance2.5--720p--301010--按次--J", Platform: PlatformJijiuSeedance, Model: "Seedance2.5按次收费J2", Version: seedanceHubVersion25, Billing: BillingModePerRequest},
	{Name: "Seedance2.5--720p--30-10--按次--J", Platform: PlatformJijiuSeedance, Model: "Seedance2.5 按次收费J1", Version: seedanceHubVersion25, Billing: BillingModePerRequest},
}

// seedanceHubMemberPlatforms are the platforms whose accounts the hub group schedules.
var seedanceHubMemberPlatforms = []string{
	PlatformCTMOSeedance,
	PlatformGuangying,
	PlatformXingAPISeedance,
	PlatformMaiTokenSeedance,
	PlatformJijiuSeedance,
}

// IsSeedanceHubMemberPlatform reports whether a platform serves lines of the hub group.
func IsSeedanceHubMemberPlatform(platform string) bool {
	platform = strings.TrimSpace(platform)
	for _, member := range seedanceHubMemberPlatforms {
		if member == platform {
			return true
		}
	}
	return false
}

// IsPPVideoGroupPlatform reports whether a group of this platform serves the video API: every
// video platform, and the Seedance aggregate group.
func IsPPVideoGroupPlatform(platform string) bool {
	return IsPPVideoPlatform(platform) || strings.TrimSpace(platform) == PlatformSeedanceHub
}

// PPVideoGroupServesPlatform reports whether a group of groupPlatform serves tasks of
// taskPlatform. A hub group serves the tasks of every member platform, because a task records
// the platform of the account that ran it.
func PPVideoGroupServesPlatform(groupPlatform, taskPlatform string) bool {
	groupPlatform = strings.TrimSpace(groupPlatform)
	taskPlatform = strings.TrimSpace(taskPlatform)
	if groupPlatform == taskPlatform {
		return true
	}
	return groupPlatform == PlatformSeedanceHub && IsSeedanceHubMemberPlatform(taskPlatform)
}

type seedanceHubIndexData struct {
	lines  []SeedanceHubLine
	byName map[string]int
}

var (
	seedanceHubOnce sync.Once
	seedanceHubData seedanceHubIndexData
)

func seedanceHubKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// seedanceHubRank orders the model list: Seedance 2.5 per request first, then 2.5 per second,
// then 2.0 per request, then 2.0 per second.
func (l SeedanceHubLine) rank() int {
	rank := 0
	if l.Version == seedanceHubVersion20 {
		rank += 2
	}
	if l.Billing == BillingModeVideo {
		rank++
	}
	return rank
}

// allNames lists every name the line answers to, public name first.
func (l SeedanceHubLine) allNames() []string {
	names := []string{l.Name, l.Model}
	for _, alias := range ppVideoPricingModelAliases(l.Platform, l.Model) {
		names = append(names, alias)
	}
	return names
}

func seedanceHubIndex() *seedanceHubIndexData {
	seedanceHubOnce.Do(func() {
		lines := append([]SeedanceHubLine(nil), seedanceHubLines...)
		sort.SliceStable(lines, func(i, j int) bool { return lines[i].rank() < lines[j].rank() })
		byName := make(map[string]int, len(lines)*3)
		for i, line := range lines {
			for _, name := range line.allNames() {
				key := seedanceHubKey(name)
				if key == "" {
					continue
				}
				if _, exists := byName[key]; !exists {
					byName[key] = i
				}
			}
		}
		seedanceHubData = seedanceHubIndexData{lines: lines, byName: byName}
	})
	return &seedanceHubData
}

// SeedanceHubLines returns the lines in model-list order. The result is a copy.
func SeedanceHubLines() []SeedanceHubLine {
	return append([]SeedanceHubLine(nil), seedanceHubIndex().lines...)
}

// SeedanceHubLineByName finds a line by any name it answers to, ignoring case.
func SeedanceHubLineByName(name string) (SeedanceHubLine, bool) {
	index := seedanceHubIndex()
	i, ok := index.byName[seedanceHubKey(name)]
	if !ok {
		return SeedanceHubLine{}, false
	}
	return index.lines[i], true
}

// SeedanceHubLineFor finds a line by any name it answers to and requires that the given
// platform serves it. Accounts use it, because a model name on an account of another platform
// does not make that account a server of the line.
func SeedanceHubLineFor(platform, model string) (SeedanceHubLine, bool) {
	line, ok := SeedanceHubLineByName(model)
	if !ok || line.Platform != strings.TrimSpace(platform) {
		return SeedanceHubLine{}, false
	}
	return line, true
}

// SeedanceHubPublicModels maps model names to the public names of their lines, without
// duplicates and in model-list order. Names that belong to no line are dropped.
func SeedanceHubPublicModels(models []string) []string {
	index := seedanceHubIndex()
	present := make(map[int]struct{}, len(models))
	for _, model := range models {
		if i, ok := index.byName[seedanceHubKey(model)]; ok {
			present[i] = struct{}{}
		}
	}
	names := make([]string, 0, len(present))
	for i, line := range index.lines {
		if _, ok := present[i]; ok {
			names = append(names, line.Name)
		}
	}
	return names
}

// ResolveSeedanceHubRequest finds the line a hub-group request asks for and returns the body
// with `model` rewritten to the name the line's platform accepts. Both the native (v1) and the
// unified (v2) body carry `model` at the top level. The error text is safe to show a client.
func ResolveSeedanceHubRequest(body []byte) (SeedanceHubLine, []byte, error) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return SeedanceHubLine{}, nil, fmt.Errorf("request body must be valid JSON")
	}
	model := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	if model == "" {
		return SeedanceHubLine{}, nil, fmt.Errorf("model is required")
	}
	line, ok := SeedanceHubLineByName(model)
	if !ok {
		return SeedanceHubLine{}, nil, fmt.Errorf("model %q is not available in this group", model)
	}
	return line, ReplaceModelInBody(body, line.Model), nil
}

// fillSeedanceHubCapabilities reports the limits of one line: the platform's own data where
// the platform has a limit table, else the documented limits recorded on the line.
func fillSeedanceHubCapabilities(capabilities *PPVideoModelCapabilities, line SeedanceHubLine) {
	switch line.Platform {
	case PlatformMaiTokenSeedance:
		fillPPVideoMaiTokenCapabilities(capabilities)
	case PlatformJijiuSeedance:
		if spec, ok := jijiuModelSpecFor(line.Model); ok {
			fillPPVideoJijiuCapabilities(capabilities, spec)
		}
	default:
		limits := line.Limits
		if limits == nil {
			return
		}
		capabilities.Duration = &PPVideoDurationRange{Min: limits.MinSeconds, Max: limits.MaxSeconds}
		capabilities.Resolutions = append([]string(nil), limits.Resolutions...)
		capabilities.Assets = &PPVideoAssetCapabilities{
			ReferenceImages: ppVideoCapInt(limits.MaxImages),
			ReferenceVideos: ppVideoCapInt(limits.MaxVideos),
			ReferenceAudios: ppVideoCapInt(limits.MaxAudios),
		}
	}
}
