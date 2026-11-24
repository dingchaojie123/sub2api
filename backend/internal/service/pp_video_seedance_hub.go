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
	// LegacyNames are earlier public names of the line. They are still accepted (and answered
	// as the line) so integrations that already use them keep working; the model list and the
	// task record show Name only.
	LegacyNames []string
	// Limits are the documented limits of the line. Only lines whose platform code has no limit
	// table of its own carry them; the others report their platform's limits.
	Limits *seedanceHubLimits
	// Hidden lines are withdrawn from the site (2026-11-17): the model lists, the model & price
	// page and the customer docs no longer show them. They stay in the table, so a client that
	// already sends the name keeps working, and showing a line again is deleting this flag.
	Hidden bool
}

type seedanceHubLimits struct {
	MinSeconds  int
	MaxSeconds  int
	Resolutions []string
	// DefaultResolution is what a request that names no resolution is billed as, when it is not the
	// first of Resolutions (the price estimate uses it). Empty means the first one.
	DefaultResolution string
	MaxImages         int
	MaxVideos         int
	MaxAudios         int
}

// seedanceHubLines is in the order the lines were agreed on: by platform, then by model. The
// order of the model list is derived from it by seedanceHubSortedLines.
var seedanceHubLines = []SeedanceHubLine{
	// 浙创 (Z). 930 and 903 are not in the operator's final table of 2026-11-19, so they are hidden
	// (still answered by name).
	{
		Name: "Seedance2.0--720p--930--按次--Z", Platform: PlatformCTMOSeedance, Model: "seedance-2.0-930",
		Version: seedanceHubVersion20, Billing: BillingModePerRequest, Hidden: true,
		Limits: &seedanceHubLimits{MinSeconds: 5, MaxSeconds: 15, Resolutions: []string{"720p"}, MaxImages: 9, MaxVideos: 3, MaxAudios: 0},
	},
	{
		Name: "Seedance2.0--720p--903--按次--Z", Platform: PlatformCTMOSeedance, Model: "seedance-2.0-903",
		Version: seedanceHubVersion20, Billing: BillingModePerRequest, Hidden: true,
		Limits: &seedanceHubLimits{MinSeconds: 5, MaxSeconds: 15, Resolutions: []string{"720p"}, MaxImages: 9, MaxVideos: 0, MaxAudios: 3},
	},
	// 933 (2026-11-17, from the operations sheet): 720p, 5-15 seconds, per request. 8 credits per
	// request since the operator's table of 2026-11-20 (5.6 before). Since the final table of
	// 2026-11-19 it takes 9 images / 3 videos / 3 audios (0 audios before). The limits are the
	// sheet's; the upstream was not asked.
	{
		Name: "Seedance2.0--720p--933--按次--Z", Platform: PlatformCTMOSeedance, Model: "seedance-2.0-933",
		Version: seedanceHubVersion20, Billing: BillingModePerRequest,
		Limits: &seedanceHubLimits{MinSeconds: 5, MaxSeconds: 15, Resolutions: []string{"720p"}, MaxImages: 9, MaxVideos: 3, MaxAudios: 3},
	},
	// kaopu-sd-2.0-mini (added 2026-11-19) was deleted on 2026-11-20: it is not in the operator's table.
	// 光影 (G)
	{Name: "Seedance2.5--720p--全参--按秒--G", Platform: PlatformGuangying, Model: "Seedance 2.5-全参720P", Version: seedanceHubVersion25, Billing: BillingModeVideo, Hidden: true},
	{Name: "Seedance2.5--720p--限时满参--按次--G", Platform: PlatformGuangying, Model: "限时限量-Seedance 2.5 满参", Version: seedanceHubVersion25, Billing: BillingModePerRequest, Hidden: true},
	{Name: "Seedance2.5--720p--限时满参备选--按次--G", Platform: PlatformGuangying, Model: "限时限量-Seedance 2.5 满参-备选", Version: seedanceHubVersion25, Billing: BillingModePerRequest, Hidden: true},
	{Name: "Seedance2.5--720p--无视频--按秒--G", Platform: PlatformGuangying, Model: "Seedance 2.5-无视频", Version: seedanceHubVersion25, Billing: BillingModeVideo, Hidden: true},
	{Name: "Seedance2.5--480p/720p--无参考视频--按秒--G", Platform: PlatformGuangying, Model: "Seedance 2.5-无参考视频 按秒计费", Version: seedanceHubVersion25, Billing: BillingModeVideo, Hidden: true},
	{Name: "Seedance2.0--720p--标准--按次--G", Platform: PlatformGuangying, Model: "Seedance 2.0按次计费", Version: seedanceHubVersion20, Billing: BillingModePerRequest, Hidden: true},
	{Name: "Seedance2.0--480p/720p/1080p--无参考视频--按秒--G", Platform: PlatformGuangying, Model: "Seedance 2.0-无参考视频 按秒计费", Version: seedanceHubVersion20, Billing: BillingModeVideo, Hidden: true},
	{Name: "Seedance2.0--720p--竖屏--按次--G", Platform: PlatformGuangying, Model: "竖屏-Seedance 2.0按次计费", Version: seedanceHubVersion20, Billing: BillingModePerRequest, Hidden: true},
	{Name: "Seedance2.0--720p--竖屏Fast--按次--G", Platform: PlatformGuangying, Model: "竖屏-Seedance 2.0 Fast 按次计费", Version: seedanceHubVersion20, Billing: BillingModePerRequest, Hidden: true},
	{Name: "Seedance2.0--720p--竖屏Mini--按次--G", Platform: PlatformGuangying, Model: "竖屏-Seedance 2.0 Mini 按次计费", Version: seedanceHubVersion20, Billing: BillingModePerRequest, Hidden: true},
	// 星光 (X)
	{
		Name: "Seedance2.0--720p--933--按次--X", Platform: PlatformXingAPISeedance, Model: "seedance-933",
		Version: seedanceHubVersion20, Billing: BillingModePerRequest, Hidden: true,
		Limits: &seedanceHubLimits{MinSeconds: 4, MaxSeconds: 15, Resolutions: []string{"720p"}, MaxImages: 9, MaxVideos: 0, MaxAudios: 3},
	},
	{
		Name: "Seedance2.0--720p--933-C--按次--X", Platform: PlatformXingAPISeedance, Model: "seedance2.0-933-C",
		Version: seedanceHubVersion20, Billing: BillingModePerRequest, Hidden: true,
		Limits: &seedanceHubLimits{MinSeconds: 4, MaxSeconds: 15, Resolutions: []string{"720p"}, MaxImages: 9, MaxVideos: 3, MaxAudios: 3},
	},
	{
		Name: "Seedance2.5--720p--V--按次--X", Platform: PlatformXingAPISeedance, Model: "seedance2.5-V",
		Version: seedanceHubVersion25, Billing: BillingModePerRequest, Hidden: true,
		Limits: &seedanceHubLimits{MinSeconds: 5, MaxSeconds: 30, Resolutions: []string{"720p"}, MaxImages: 30, MaxVideos: 10, MaxAudios: 10},
	},
	// Mai Token (M)
	{Name: "Seedance2.5--480p--全参--按秒--M", Platform: PlatformMaiTokenSeedance, Model: MaiTokenSeedancePublicModel, Version: seedanceHubVersion25, Billing: BillingModeVideo},
	// 极九 (J)
	{Name: "Seedance2.5--1080p--全参C--按秒--J", Platform: PlatformJijiuSeedance, Model: "满血seedance-2.5全参C", Version: seedanceHubVersion25, Billing: BillingModeVideo, Hidden: true},
	{Name: "Seedance2.5--480p/720p/1080p--全参D--按秒--J", Platform: PlatformJijiuSeedance, Model: "稳定seedance-2.5全参D", Version: seedanceHubVersion25, Billing: BillingModeVideo},
	{Name: "Seedance2.5--480p--参图参音F--按秒--J", Platform: PlatformJijiuSeedance, Model: "稳定seedance-2.5参图参音F", Version: seedanceHubVersion25, Billing: BillingModeVideo},
	{Name: "Seedance2.0--480p/720p/1080p--933线路一--按秒--J", Platform: PlatformJijiuSeedance, Model: "SD2.0满血稳定933-线路一", Version: seedanceHubVersion20, Billing: BillingModeVideo, Hidden: true},
	// 903z is billed per request in the channel (the operations sheet of 2026-11-10), although the
	// upstream bills it per second, so the billing segment of the name says 按次.
	{Name: "Seedance2.0--720p--903z--按次--J", Platform: PlatformJijiuSeedance, Model: "SD2.0特价稳定903z", Version: seedanceHubVersion20, Billing: BillingModePerRequest, Hidden: true},
	{Name: "Seedance2.0--720p--903mini--按次--J", Platform: PlatformJijiuSeedance, Model: "SD2.0mini特价稳定903z", Version: seedanceHubVersion20, Billing: BillingModePerRequest, Hidden: true},
	{Name: "Seedance2.5--720p--301010--按次--J", Platform: PlatformJijiuSeedance, Model: "Seedance2.5按次收费J2", Version: seedanceHubVersion25, Billing: BillingModePerRequest, Hidden: true},
	{Name: "Seedance2.5--720p--30-10--按次--J", Platform: PlatformJijiuSeedance, Model: "Seedance2.5 按次收费J1", Version: seedanceHubVersion25, Billing: BillingModePerRequest, Hidden: true},
	// 喵呜 (W)
	// 2026-11-19 final table: the visible 喵呜 lines were exactly slow (特价满血), pro, flash, flash-s,
	// doubao-fast and doubao-mini; deal was added back on 2026-11-24. fast-deal, fast, 2.0-tj, 2.0-pro,
	// 2.5 flash-tj / tj / wx are hidden
	// (still answered by name; to show one again delete its Hidden flag).
	// slow takes 480p or 720p since 2026-11-12 (the name used to say 720p only; the old name is
	// kept as a legacy name, but its channel price must be re-entered under the new name).
	// Renamed 2026-11-18 from "...--slow--按秒--W" to "...--特价满血--按秒--W" (display name only; the
	// model stays seedance-2.5-slow, because 星光 already has a line whose model is
	// seedance-2.5-特价满血). The previous name is NOT kept as a legacy name (operator's decision),
	// and its channel price must be re-entered under the new name.
	{Name: "Seedance2.5--480p/720p--特价满血--按秒--W", Platform: PlatformMiaowuSeedance, Model: "seedance-2.5-slow", Version: seedanceHubVersion25, Billing: BillingModeVideo, LegacyNames: []string{"Seedance2.5--720p--slow--按秒--W"}},
	// deal was hidden by the 2026-11-19 sweep and is VISIBLE again since 2026-11-24 (the operator's
	// request): 720p only, per second (1.2 credits), 4-30 seconds, 30 images / no videos / 10 audios.
	{Name: "Seedance2.5--720p--deal--按秒--W", Platform: PlatformMiaowuSeedance, Model: "seedance-2.5-deal", Version: seedanceHubVersion25, Billing: BillingModeVideo},
	{Name: "Seedance2.5--480p/720p--flash--按次--W", Platform: PlatformMiaowuSeedance, Model: "seedance-2.5-flash", Version: seedanceHubVersion25, Billing: BillingModePerRequest},
	{Name: "Seedance2.5--720p--pro--按次--W", Platform: PlatformMiaowuSeedance, Model: "seedance-2.5-pro", Version: seedanceHubVersion25, Billing: BillingModePerRequest},
	// Seedance 2.0 lines of 喵呜 (2026-11-13). deal is per request but priced per resolution
	// (480p 4, 720p 8 credits); fast is per request (4.5 credits) since 2026-11-17.
	{Name: "Seedance2.0--720p--fast-deal--按次--W", Platform: PlatformMiaowuSeedance, Model: "seedance-2.0-fast-deal", Version: seedanceHubVersion20, Billing: BillingModePerRequest, Hidden: true},
	// fast is billed per request (4.5 credits) since 2026-11-17; the per-second name is kept as a
	// legacy name, but its channel price must be re-entered under the new name.
	{Name: "Seedance2.0--480p/720p--fast--按次--W", Platform: PlatformMiaowuSeedance, Model: "seedance-2.0-fast", Version: seedanceHubVersion20, Billing: BillingModePerRequest, Hidden: true, LegacyNames: []string{"Seedance2.0--480p/720p--fast--按秒--W"}},
	{Name: "Seedance2.0--720p--916--按次--W", Platform: PlatformMiaowuSeedance, Model: "seedance-2.0-916", Version: seedanceHubVersion20, Billing: BillingModePerRequest, Hidden: true},
	{Name: "Seedance2.0--480p/720p--deal--按次--W", Platform: PlatformMiaowuSeedance, Model: "seedance-2.0-deal", Version: seedanceHubVersion20, Billing: BillingModePerRequest, Hidden: true},
	// tj / pro added 2026-11-17: tj is per request (720p), pro is per second with a price per
	// resolution (720p / 1080p / 4k).
	// Visible again since 2026-11-18: they were hidden by the 11-17 sweep, so a dedicated account that
	// had both models configured did not get them in /v1/models.
	// Since 2026-11-21 tj takes 480p / 720p / 1080p / 4k and 3 reference videos (the operator's request),
	// so its line name changed from "...--720p--tj--按次--W"; the old name is kept as a legacy name, but
	// its channel price must be re-entered under the new name. fast-tj and mini-tj (same spec, per
	// request) were added the same day. All three are VISIBLE (the operator's request, 2026-11-21).
	{Name: "Seedance2.0--480p/720p/1080p/4k--tj--按次--W", Platform: PlatformMiaowuSeedance, Model: "seedance-2.0-tj", Version: seedanceHubVersion20, Billing: BillingModePerRequest, LegacyNames: []string{"Seedance2.0--720p--tj--按次--W"}},
	{Name: "Seedance2.0--480p/720p/1080p/4k--fast-tj--按次--W", Platform: PlatformMiaowuSeedance, Model: "seedance-2.0-fast-tj", Version: seedanceHubVersion20, Billing: BillingModePerRequest},
	{Name: "Seedance2.0--480p/720p/1080p/4k--mini-tj--按次--W", Platform: PlatformMiaowuSeedance, Model: "seedance-2.0-mini-tj", Version: seedanceHubVersion20, Billing: BillingModePerRequest},
	{Name: "Seedance2.0--720p/1080p/4k--pro--按秒--W", Platform: PlatformMiaowuSeedance, Model: "seedance-2.0-pro", Version: seedanceHubVersion20, Billing: BillingModeVideo, Hidden: true},
	// 2.5 tj lines added 2026-11-18, visible (the operator sent them with prices): flash-tj is
	// 720p at 15 credits per request, tj is per request with 480p 10 / 720p 19 credits.
	{Name: "Seedance2.5--720p--flash-tj--按次--W", Platform: PlatformMiaowuSeedance, Model: "seedance-2.5-flash-tj", Version: seedanceHubVersion25, Billing: BillingModePerRequest, Hidden: true},
	{Name: "Seedance2.5--480p/720p--tj--按次--W", Platform: PlatformMiaowuSeedance, Model: "seedance-2.5-tj", Version: seedanceHubVersion25, Billing: BillingModePerRequest, Hidden: true},
	// wx added 2026-11-18, visible: same limits and prices as tj (480p 10 / 720p 19 credits per request).
	{Name: "Seedance2.5--480p/720p--wx--按次--W", Platform: PlatformMiaowuSeedance, Model: "seedance-2.5-wx", Version: seedanceHubVersion25, Billing: BillingModePerRequest, Hidden: true},
	// doubao-seedance-2.0-fast / -mini added 2026-11-19, visible (the operator sent them with prices):
	// both are 720p only, per request (fast 5, mini 3.5 credits since 2026-11-20; 4.5 / 2.5 before), 9 images / no videos / no audios.
	{Name: "Seedance2.0--720p--doubao-fast--按次--W", Platform: PlatformMiaowuSeedance, Model: "doubao-seedance-2.0-fast", Version: seedanceHubVersion20, Billing: BillingModePerRequest},
	{Name: "Seedance2.0--720p--doubao-mini--按次--W", Platform: PlatformMiaowuSeedance, Model: "doubao-seedance-2.0-mini", Version: seedanceHubVersion20, Billing: BillingModePerRequest},
	// seedance-2.5-flash-s added 2026-11-19, visible: per second (480p 0.6 / 720p 1 credit since 2026-11-20; 0.4 / 0.7 before), 4-30
	// seconds, 10 images / no videos / 5 audios.
	{Name: "Seedance2.5--480p/720p--flash-s--按秒--W", Platform: PlatformMiaowuSeedance, Model: "seedance-2.5-flash-s", Version: seedanceHubVersion25, Billing: BillingModeVideo},
	// Paisio (P): the model segment keeps the descriptions the operator gave (满血原生过人脸 /
	// 原生过人脸); the 720p model takes no reference videos.
	{Name: "Seedance2.5--480p--满血原生过人脸--按秒--P", Platform: PlatformPaisioSeedance, Model: "paisio-seedance-2.5-480p", Version: seedanceHubVersion25, Billing: BillingModeVideo, Hidden: true},
	{Name: "Seedance2.5--720p--原生过人脸--按秒--P", Platform: PlatformPaisioSeedance, Model: "paisio-seedance-2.5-720p", Version: seedanceHubVersion25, Billing: BillingModeVideo, Hidden: true},
	// Apophis (A): the third-party lines of the upstream site's capability table. The model
	// segment is the line code at the end of the upstream name; the lines marked 按次 are billed
	// per request, the others per second. The token-billed official-forward lines are not served.
	{Name: "Seedance2.0--720p--903--按次--A", Platform: PlatformApophisSeedance, Model: "Seedance2.0-720p-按次-903", Version: seedanceHubVersion20, Billing: BillingModePerRequest, Hidden: true},
	{Name: "Seedance2.0--720p--gf--按秒--A", Platform: PlatformApophisSeedance, Model: "Seedance2.0-720p-按秒-gf", Version: seedanceHubVersion20, Billing: BillingModeVideo, Hidden: true},
	{Name: "Seedance2.0--720p--X--按次--A", Platform: PlatformApophisSeedance, Model: "Seedance2.0-720p-按次-X", Version: seedanceHubVersion20, Billing: BillingModePerRequest, Hidden: true},
	{Name: "Seedance2.0--720p--Xs--按秒--A", Platform: PlatformApophisSeedance, Model: "Seedance2.0-720p-按秒-Xs", Version: seedanceHubVersion20, Billing: BillingModeVideo, Hidden: true},
	{Name: "Seedance2.0--720p--Xs--按次--A", Platform: PlatformApophisSeedance, Model: "Seedance2.0-720p-按次-Xs", Version: seedanceHubVersion20, Billing: BillingModePerRequest, Hidden: true},
	{Name: "Seedance2.0--720p--max--按秒--A", Platform: PlatformApophisSeedance, Model: "Seedance2.0-720p-按秒-max", Version: seedanceHubVersion20, Billing: BillingModeVideo, Hidden: true},
	{Name: "Seedance2.5--720p--9图--按秒--A", Platform: PlatformApophisSeedance, Model: "Seedance2.5-720p-按秒-9图", Version: seedanceHubVersion25, Billing: BillingModeVideo, Hidden: true},
	{Name: "Seedance2.5--1080p--Y--按次--A", Platform: PlatformApophisSeedance, Model: "Seedance2.5-1080p-按次-Y", Version: seedanceHubVersion25, Billing: BillingModePerRequest, Hidden: true},
	{Name: "Seedance2.5--480p--Z--按秒--A", Platform: PlatformApophisSeedance, Model: "Seedance2.5-480p-按秒-Z", Version: seedanceHubVersion25, Billing: BillingModeVideo, Hidden: true},
	{Name: "Seedance2.5--720p--Z--按秒--A", Platform: PlatformApophisSeedance, Model: "Seedance2.5-720p-按秒-Z", Version: seedanceHubVersion25, Billing: BillingModeVideo, Hidden: true},
	{Name: "Seedance2.5--720p--Z满血--按次--A", Platform: PlatformApophisSeedance, Model: "Seedance2.5-720p-按次-Z满血", Version: seedanceHubVersion25, Billing: BillingModePerRequest, Hidden: true},
	{Name: "Seedance2.5--720p--301010--按秒--A", Platform: PlatformApophisSeedance, Model: "Seedance2.5-720p-按秒-301010", Version: seedanceHubVersion25, Billing: BillingModeVideo, Hidden: true},
	{Name: "Seedance2.5--720p--301010--按次--A", Platform: PlatformApophisSeedance, Model: "Seedance2.5-720p-按次-301010", Version: seedanceHubVersion25, Billing: BillingModePerRequest, Hidden: true},
	{Name: "Seedance2.5--720p--gz--按次--A", Platform: PlatformApophisSeedance, Model: "Seedance2.5-720p-按次-gz", Version: seedanceHubVersion25, Billing: BillingModePerRequest, Hidden: true},
	{Name: "Seedance2.5--720p--满血全参--按次--A", Platform: PlatformApophisSeedance, Model: "Seedance2.5-720p-按次-满血全参", Version: seedanceHubVersion25, Billing: BillingModePerRequest, Hidden: true},
	// XSN (N): fixed 720p. 特价满血 is billed per output second; reference videos are refused (the
	// upstream doubles its price for them and the channel has one price per line).
	{Name: "Seedance2.5--720p--特价满血--按秒--N", Platform: PlatformXSNSeedance, Model: "seedance-2.5-特价满血", Version: seedanceHubVersion25, Billing: BillingModeVideo, Hidden: true},
	// 全天打 (2026-11-14): 5-30 s, billed per request whatever the length (19 credits, set in the
	// channel); also refuses reference videos.
	{Name: "Seedance2.5--720p--全天打--按次--N", Platform: PlatformXSNSeedance, Model: "seedance-2.5-全天打", Version: seedanceHubVersion25, Billing: BillingModePerRequest, Hidden: true},
	// 即梦 (2026-11-16): official forward with faces. Sd-2.5 is per second (480p 1, 720p 1.2 credits
	// per second, two tiers in the channel); flash is per request (17 credits). Both take
	// reference videos (10 and 5).
	{Name: "Seedance2.5--480p/720p--即梦-Sd-2.5--按秒--N", Platform: PlatformXSNSeedance, Model: XSNJimengPublicModel, Version: seedanceHubVersion25, Billing: BillingModeVideo},
	{Name: "Seedance2.5--720p--即梦-Sd-2.5-flash--按次--N", Platform: PlatformXSNSeedance, Model: XSNJimengFlashPublicModel, Version: seedanceHubVersion25, Billing: BillingModePerRequest},
	// Fengyue: one model, 5-30 seconds, 720p only, billed per request at one flat channel price for
	// every length (the operator set 19 credits; the price itself lives in the channel). The
	// upstream prices by length band (5-15 s and 16-30 s), which this line deliberately does not
	// mirror. The name is the operator's own (2026-11-12) and does not follow the five-segment
	// pattern of the other lines: "Seedance 2.5--720p--按次--FY" (a space after Seedance, no model
	// segment, platform tag FY); the line table test lists it as an exception.
	{Name: "Seedance 2.5--720p--按次--FY", Platform: PlatformFengyueSeedance, Model: FengyueSeedancePublicModel, Version: seedanceHubVersion25, Billing: BillingModePerRequest},
	// Hongpei (H, 2026-11-14): four models, all billed per request whatever the length (the
	// upstream bills per video too); the prices live in the channel. L, v and Y are 720p only,
	// flash takes 480p or 720p at one price. Ratios, prompt limits and reference limits differ per
	// model and are enforced by the platform's own table (pp_video_hongpei.go).
	// Model is the public name; the upstream names (vs-sd2.5-L, ...) are still accepted in the hub.
	// L is VISIBLE since 2026-11-24 (the operator's request: "不要隐藏", still in use); v, Y and flash stay hidden.
	{Name: "Seedance2.5--720p--L--按次--H", Platform: PlatformHongpeiSeedance, Model: "Seedance2.5-vs-L", Version: seedanceHubVersion25, Billing: BillingModePerRequest},
	{Name: "Seedance2.5--720p--v--按次--H", Platform: PlatformHongpeiSeedance, Model: "Seedance2.5-vs-v", Version: seedanceHubVersion25, Billing: BillingModePerRequest, Hidden: true},
	{Name: "Seedance2.5--720p--Y--按次--H", Platform: PlatformHongpeiSeedance, Model: "Seedance2.5-vs-Y", Version: seedanceHubVersion25, Billing: BillingModePerRequest, Hidden: true},
	{Name: "Seedance2.5--480p/720p--flash--按次--H", Platform: PlatformHongpeiSeedance, Model: "Seedance2.5-vs-flash", Version: seedanceHubVersion25, Billing: BillingModePerRequest, Hidden: true},
	// vs-sd2.5-a (2026-11-20, from the operator): 480p / 720p / 1080p / 4k, per request, one channel
	// tier per resolution (11 / 19 / 27 / 35 credits), 4-30 seconds, 30 images / 10 videos / 10 audios.
	// Hidden like the other H lines (still answered by name); showing it is deleting the flag. The model
	// segment "a" is the upstream's own suffix.
	{Name: "Seedance2.5--480p/720p/1080p/4k--a--按次--H", Platform: PlatformHongpeiSeedance, Model: "Seedance2.5-vs-a", Version: seedanceHubVersion25, Billing: BillingModePerRequest, Hidden: true},
	// vs-sd2.5-s / -nh / -X (2026-11-21): s and X per request (720p), nh per SECOND (480p / 720p / 1080p, one
	// channel tier each, mode "video"). VISIBLE since 2026-11-21 (the operator's request); the other H lines stay hidden.
	{Name: "Seedance2.5--720p--s--按次--H", Platform: PlatformHongpeiSeedance, Model: "Seedance2.5-vs-s", Version: seedanceHubVersion25, Billing: BillingModePerRequest},
	{Name: "Seedance2.5--480p/720p/1080p--nh--按秒--H", Platform: PlatformHongpeiSeedance, Model: "Seedance2.5-vs-nh", Version: seedanceHubVersion25, Billing: BillingModeVideo},
	{Name: "Seedance2.5--720p--X--按次--H", Platform: PlatformHongpeiSeedance, Model: "Seedance2.5-vs-X", Version: seedanceHubVersion25, Billing: BillingModePerRequest},
	// HUAHUO (HH, 2026-11-16): three stable full-reference (满参) Seedance 2.5 models. The model
	// segment keeps the operator's description (满参稳定); the billing segment says which are priced
	// per second and which per request (the prices live in the channel). The resolutions, lengths
	// and reference counts are assumptions until checked against the upstream's catalogue; they are
	// enforced by the platform's own table (pp_video_huahuo.go), and the resolutions in these names
	// must follow that table (the line table test checks it).
	{Name: "Seedance2.5--720p--满参稳定--按秒--HH", Platform: PlatformHuahuoSeedance, Model: "Seedance2.5-mancan-anmiao", Version: seedanceHubVersion25, Billing: BillingModeVideo},
	{Name: "Seedance2.5--720p--满参稳定--按次--HH", Platform: PlatformHuahuoSeedance, Model: "Seedance2.5-mancan-anci", Version: seedanceHubVersion25, Billing: BillingModePerRequest},
	{Name: "Seedance2.5--480p/720p--flash满参稳定--按秒--HH", Platform: PlatformHuahuoSeedance, Model: "Seedance2.5-mancan-flash", Version: seedanceHubVersion25, Billing: BillingModeVideo, Hidden: true},
	// video-2.5shuliang1 (2026-11-19, from the operator), visible: per request (19 credits), 720p,
	// 5-30 seconds, 30 images / 10 videos / 10 audios. The model segment and the public name are mine.
	{Name: "Seedance2.5--720p--shuliang1--按次--HH", Platform: PlatformHuahuoSeedance, Model: "Seedance2.5-shuliang1", Version: seedanceHubVersion25, Billing: BillingModePerRequest},
	// VSHI (V, 2026-11-16): four models, all billed per second (the operator's decision; the
	// prices live in the channel). The resolutions in these names follow the platform's own table
	// (pp_video_vshi.go, checked by the line table test). The upstream names (NSFW-pro, ...) are
	// still accepted in the hub; the lines carry the public names.
	{Name: "Seedance2.0--480p/720p/1080p/4k--pro--按秒--V", Platform: PlatformVshiSeedance, Model: "Seedance2.0-NSFW-pro", Version: seedanceHubVersion20, Billing: BillingModeVideo, Hidden: true},
	{Name: "Seedance2.0--480p/720p--fast--按秒--V", Platform: PlatformVshiSeedance, Model: "Seedance2.0-NSFW-fast", Version: seedanceHubVersion20, Billing: BillingModeVideo, Hidden: true},
	{Name: "Seedance2.0--480p/720p--mini--按秒--V", Platform: PlatformVshiSeedance, Model: "Seedance2.0-NSFW-mini", Version: seedanceHubVersion20, Billing: BillingModeVideo, Hidden: true},
	{Name: "Seedance2.5--480p/720p/1080p--NSFW--按秒--V", Platform: PlatformVshiSeedance, Model: "Seedance2.5-NSFW", Version: seedanceHubVersion25, Billing: BillingModeVideo, Hidden: true},
	// 泽西 (ZX, 2026-11-16): four models, all billed per second (the operator's decision; the prices
	// live in the channel). The model segment is the part of the upstream name after "doubao-seedance-
	// 2.x-" (nsfw-95 / nsfw). The letter Z is taken by 浙创, so the platform tag is ZX. The resolutions in
	// these names follow the platform's own table (pp_video_zexi.go, checked by the line table test).
	// Model is the upstream name, which is also the public name on this platform.
	{Name: "Seedance2.5--480p/720p/1080p--nsfw-95--按秒--ZX", Platform: PlatformZexiSeedance, Model: "doubao-seedance-2.5-nsfw-95", Version: seedanceHubVersion25, Billing: BillingModeVideo},
	{Name: "Seedance2.5--480p/720p/1080p--nsfw--按秒--ZX", Platform: PlatformZexiSeedance, Model: "doubao-seedance-2.5-nsfw", Version: seedanceHubVersion25, Billing: BillingModeVideo},
	{Name: "Seedance2.0--480p/720p/1080p/4k--nsfw-95--按秒--ZX", Platform: PlatformZexiSeedance, Model: "doubao-seedance-2.0-nsfw-95", Version: seedanceHubVersion20, Billing: BillingModeVideo},
	{Name: "Seedance2.0--480p/720p/1080p/4k--nsfw--按秒--ZX", Platform: PlatformZexiSeedance, Model: "doubao-seedance-2.0-nsfw", Version: seedanceHubVersion20, Billing: BillingModeVideo},
	// xm API (XM, 2026-11-17): three Seedance 2.5 models, 4-30 seconds. The model segment keeps the
	// operator's own labels: 稳定满参真人脸 (稳定渠道-满参-sd2.5 真人脸, 720p, billed per request,
	// no reference videos; renamed from 480p/720p/1080p--按秒 on 2026-11-18), 01 (sd2.5-01-720,
	// billed per request) and M满参 (SD2.5-M-满参, billed per request); the prices live in the channel. The resolutions in these names follow the
	// platform's own table (pp_video_xm.go, checked by the line table test). Model is the public
	// name; the upstream ids (seedance_2.5, ...) are still accepted in the hub.
	// 2026-11-19 final table: seedance_2.5 (stable-face) is in it, so its line is visible again.
	{Name: "Seedance2.5--720p--稳定满参真人脸--按次--XM", Platform: PlatformXMSeedance, Model: "Seedance2.5-XM-stable-face", Version: seedanceHubVersion25, Billing: BillingModePerRequest},
	{Name: "Seedance2.5--720p--01--按次--XM", Platform: PlatformXMSeedance, Model: "Seedance2.5-XM-720p-vip", Version: seedanceHubVersion25, Billing: BillingModePerRequest, Hidden: true},
	{Name: "Seedance2.5--720p--M满参--按次--XM", Platform: PlatformXMSeedance, Model: "Seedance2.5-XM-M-full", Version: seedanceHubVersion25, Billing: BillingModePerRequest, Hidden: true},
	// seedance_2.0 (2026-11-19, from the operator), visible: per second (480p 0.5 / 720p 0.8 / 1080p
	// 1.2 credits since 2026-11-20; 0.3 / 0.4 / 0.7 before), 5-15 seconds, 9 images / no videos / 3 audios. The public name is mine.
	{Name: "Seedance2.0--480p/720p/1080p--sd2.0--按秒--XM", Platform: PlatformXMSeedance, Model: "Seedance2.0-XM", Version: seedanceHubVersion20, Billing: BillingModeVideo},
	// seedance_2.5-flash (2026-11-19, from the operator), visible: per second (480p 0.6 / 720p 1
	// credit since 2026-11-20; 0.4 / 0.7 before), 4-30 seconds, 10 images / no videos / 5 audios. The public name is mine.
	{Name: "Seedance2.5--480p/720p--flash--按秒--XM", Platform: PlatformXMSeedance, Model: "Seedance2.5-XM-flash", Version: seedanceHubVersion25, Billing: BillingModeVideo},
	// SD2.5-X-满参支持人脸 (2026-11-24, from the operator), visible: per request (19 credits), 720p,
	// 5-30 seconds, 30 images / 10 videos / 10 audios. The model segment X满参 follows M满参; the public name is mine.
	{Name: "Seedance2.5--720p--X满参--按次--XM", Platform: PlatformXMSeedance, Model: "Seedance2.5-XM-X-full", Version: seedanceHubVersion25, Billing: BillingModePerRequest},
	// 奶蛙 (NW, 2026-11-17): three Seedance 2.5 models, all billed per request (the prices live in the
	// channel; the two M models are priced per resolution). The model segment is the part of the
	// upstream id after "奶蛙sd2.5-" (V / flash-M / M). The letter N is taken by XSN, so the platform tag
	// is NW. The resolutions in these names follow the platform's own table (pp_video_naiwa.go, checked
	// by the line table test). Model is the public name; the upstream ids (奶蛙sd2.5-V, ...) are still
	// accepted in the hub.
	{Name: "Seedance2.5--720p--V--按次--NW", Platform: PlatformNaiwaSeedance, Model: "Seedance2.5-NW-V", Version: seedanceHubVersion25, Billing: BillingModePerRequest, Hidden: true},
	{Name: "Seedance2.5--480p/720p--flash-M--按次--NW", Platform: PlatformNaiwaSeedance, Model: "Seedance2.5-NW-flash-M", Version: seedanceHubVersion25, Billing: BillingModePerRequest, Hidden: true},
	{Name: "Seedance2.5--480p/720p--M--按次--NW", Platform: PlatformNaiwaSeedance, Model: "Seedance2.5-NW-M", Version: seedanceHubVersion25, Billing: BillingModePerRequest, Hidden: true},
	// V6K (www.v6k.top, 2026-11-21): three Seedance 2.0 models, all billed per request (the prices live
	// in the channel: 11 / 5 / 4 credits), 4-15 seconds, 9 images / no videos / 3 audios. The model
	// segment is the operator's own label (官方按条 = 官方按条SD2.0, 官方fast = 官方sd2.0-fast, mini =
	// SD2.0-mini); the platform tag is V6K because V is taken by VSHI. The resolutions in these names
	// follow the platform's own table (pp_video_v6k.go, checked by the line table test). Model is the
	// public name; the upstream ids (官方按条SD2.0, ...) are still accepted in the hub. All three lines
	// are VISIBLE (the operator's request, 2026-11-21; first added hidden).
	{Name: "Seedance2.0--480p/720p/1080p/4k--官方按条--按次--V6K", Platform: PlatformV6KSeedance, Model: "Seedance2.0-V6K", Version: seedanceHubVersion20, Billing: BillingModePerRequest},
	{Name: "Seedance2.0--480p/720p--官方fast--按次--V6K", Platform: PlatformV6KSeedance, Model: "Seedance2.0-V6K-fast", Version: seedanceHubVersion20, Billing: BillingModePerRequest},
	{Name: "Seedance2.0--480p/720p--mini--按次--V6K", Platform: PlatformV6KSeedance, Model: "Seedance2.0-V6K-mini", Version: seedanceHubVersion20, Billing: BillingModePerRequest},
	// V6K 活动 (2026-11-23): Seedance 2.5, 720p only, 4-30 seconds, 30 images / 10 videos / 10 audios,
	// billed per request (channel: 19 credits). The model segment is the operator's own label (活动);
	// the upstream id is seedanco2.5-720 (see pp_video_v6k.go). VISIBLE (the operator's request).
	{Name: "Seedance2.5--720p--活动--按次--V6K", Platform: PlatformV6KSeedance, Model: "Seedance2.5-V6K-720", Version: seedanceHubVersion25, Billing: BillingModePerRequest},
}

// seedanceHubMemberPlatforms are the platforms whose accounts the hub group schedules.
var seedanceHubMemberPlatforms = []string{
	PlatformCTMOSeedance,
	PlatformGuangying,
	PlatformXingAPISeedance,
	PlatformMaiTokenSeedance,
	PlatformJijiuSeedance,
	PlatformMiaowuSeedance,
	PlatformPaisioSeedance,
	PlatformApophisSeedance,
	PlatformXSNSeedance,
	PlatformFengyueSeedance,
	PlatformHongpeiSeedance,
	PlatformHuahuoSeedance,
	PlatformVshiSeedance,
	PlatformZexiSeedance,
	PlatformXMSeedance,
	PlatformNaiwaSeedance,
	PlatformV6KSeedance,
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
// taskPlatform. A task records the platform of the account that ran it, so a hub group serves the
// tasks of every member platform, and the ordinary group of a member platform serves them too: a
// line route can have another member platform's account run a request of that group.
func PPVideoGroupServesPlatform(groupPlatform, taskPlatform string) bool {
	groupPlatform = strings.TrimSpace(groupPlatform)
	taskPlatform = strings.TrimSpace(taskPlatform)
	if groupPlatform == taskPlatform {
		return true
	}
	if !IsSeedanceHubMemberPlatform(taskPlatform) {
		return false
	}
	return groupPlatform == PlatformSeedanceHub || IsSeedanceHubMemberPlatform(groupPlatform)
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
	names = append(names, l.LegacyNames...)
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
// duplicates and in model-list order. Names that belong to no line, and hidden lines, are dropped.
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
		if line.Hidden {
			continue
		}
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
	case PlatformMiaowuSeedance:
		if spec, ok := miaowuModelSpecFor(line.Model); ok {
			fillPPVideoMiaowuCapabilities(capabilities, spec)
		}
	case PlatformPaisioSeedance:
		if spec, ok := paisioModelSpecFor(line.Model); ok {
			fillPPVideoPaisioCapabilities(capabilities, spec)
		}
	case PlatformApophisSeedance:
		if spec, ok := apophisModelSpecFor(line.Model); ok {
			fillPPVideoApophisCapabilities(capabilities, spec)
		}
	case PlatformXSNSeedance:
		if spec, ok := xsnModelSpecFor(line.Model); ok {
			fillPPVideoXSNCapabilities(capabilities, spec)
		}
	case PlatformFengyueSeedance:
		if spec, ok := fengyueModelSpecFor(line.Model); ok {
			fillPPVideoFengyueCapabilities(capabilities, spec)
		}
	case PlatformHongpeiSeedance:
		if entry, ok := hongpeiModelFor(line.Model); ok {
			fillPPVideoHongpeiCapabilities(capabilities, entry)
		}
	case PlatformHuahuoSeedance:
		if spec, ok := huahuoModelSpecFor(line.Model); ok {
			fillPPVideoHuahuoCapabilities(capabilities, spec)
		}
	case PlatformVshiSeedance:
		if spec, ok := vshiModelSpecFor(line.Model); ok {
			fillPPVideoVshiCapabilities(capabilities, spec)
		}
	case PlatformZexiSeedance:
		if spec, ok := zexiModelSpecFor(line.Model); ok {
			fillPPVideoZexiCapabilities(capabilities, spec)
		}
	case PlatformXMSeedance:
		if spec, ok := xmModelSpecFor(line.Model); ok {
			fillPPVideoXMCapabilities(capabilities, spec)
		}
	case PlatformNaiwaSeedance:
		if spec, ok := naiwaModelSpecFor(line.Model); ok {
			fillPPVideoNaiwaCapabilities(capabilities, spec)
		}
	case PlatformV6KSeedance:
		if spec, ok := v6kModelSpecFor(line.Model); ok {
			fillPPVideoV6KCapabilities(capabilities, spec)
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
