package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// Line routes of Seedance aggregate groups: lookup at request time, validation when an operator
// saves a route, and the price of a routed request.

const (
	lineRouteW   = "Seedance2.5--720p--pro--按次--W"
	lineRouteHH  = "Seedance2.5--720p--满参稳定--按次--HH"
	lineRouteXM  = "Seedance2.5--720p--稳定满参真人脸--按次--XM"
	lineRouteW20 = "Seedance2.0--720p--doubao-fast--按次--W"
	// A line withdrawn from the site (Hidden): it cannot be the target of a route.
	lineRouteHidden = "Seedance2.5--720p--flash-tj--按次--W"
)

// lineRouteStoreStub is the route storage and the group lookup; it is the usage billing
// repository of the service under test (only the route capability is used).
type lineRouteStoreStub struct {
	UsageBillingRepository

	mu       sync.Mutex
	platform string
	routes   map[string]string
	listErr  error
	lists    int

	// The global switch is ON unless switchOff is set (the real default is off; the tests that are
	// not about the switch need routing to apply).
	switchOff   bool
	switchErr   error
	switchReads int
}

// resetLineRouteCaches drops the cached routes of a group and the cached global switch, so a test
// sees the state of its own stub.
func resetLineRouteCaches(groupID int64) {
	invalidatePPVideoLineRouteCache(groupID)
	invalidatePPVideoLineRoutesSwitchCache()
}

func (r *lineRouteStoreStub) ListPPVideoLineRoutes(_ context.Context, _ int64) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lists++
	if r.listErr != nil {
		return nil, r.listErr
	}
	out := make(map[string]string, len(r.routes))
	for k, v := range r.routes {
		out[k] = v
	}
	return out, nil
}

func (r *lineRouteStoreStub) SetPPVideoLineRoute(_ context.Context, _ int64, line, target string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.routes == nil {
		r.routes = map[string]string{}
	}
	r.routes[line] = target
	return nil
}

func (r *lineRouteStoreStub) DeletePPVideoLineRoute(_ context.Context, _ int64, line string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.routes, line)
	return nil
}

func (r *lineRouteStoreStub) GetPPVideoLineRoutesEnabled(_ context.Context) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.switchReads++
	if r.switchErr != nil {
		return false, r.switchErr
	}
	return !r.switchOff, nil
}

func (r *lineRouteStoreStub) SetPPVideoLineRoutesEnabled(_ context.Context, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.switchOff = !enabled
	return nil
}

func (r *lineRouteStoreStub) GetPPVideoLineRouteGroupPlatform(_ context.Context, _ int64) (string, error) {
	if r.platform == "" {
		return "", ErrPPVideoLineRouteGroupNotFound
	}
	return r.platform, nil
}

// lineRouteAccountRepoStub lists the schedulable accounts of the group under test.
type lineRouteAccountRepoStub struct {
	AccountRepository
	accounts []Account
}

func (r lineRouteAccountRepoStub) ListSchedulableByGroupID(_ context.Context, _ int64) ([]Account, error) {
	return r.accounts, nil
}

func lineRouteAccount(id int64, platform string, models ...string) Account {
	mapping := make(map[string]any, len(models))
	for _, model := range models {
		mapping[model] = model
	}
	return Account{ID: id, Platform: platform, Credentials: map[string]any{"model_mapping": mapping}}
}

// lineRouteService builds a service whose group has a 喵呜 account (W pro, W doubao-fast) and a
// 花火 account (HH 满参稳定 按次), the shape of a customer's exclusive group.
func lineRouteService(store *lineRouteStoreStub) *OpenAIGatewayService {
	return &OpenAIGatewayService{
		usageBillingRepo: store,
		accountRepo: lineRouteAccountRepoStub{accounts: []Account{
			lineRouteAccount(1, PlatformMiaowuSeedance, "seedance-2.5-pro", "doubao-seedance-2.0-fast"),
			lineRouteAccount(2, PlatformHuahuoSeedance, "Seedance2.5-mancan-anci"),
		}},
	}
}

func mustHubLine(t *testing.T, name string) SeedanceHubLine {
	t.Helper()
	line, ok := SeedanceHubLineByName(name)
	require.True(t, ok, name)
	return line
}

func TestLineRouteFixturesAreRealLines(t *testing.T) {
	t.Parallel()

	for _, name := range []string{lineRouteW, lineRouteHH, lineRouteXM, lineRouteW20} {
		require.False(t, mustHubLine(t, name).Hidden, name)
	}
	require.True(t, mustHubLine(t, lineRouteHidden).Hidden)
}

func TestResolvePPVideoLineRoute(t *testing.T) {
	const groupID = int64(910001)
	t.Cleanup(func() { resetLineRouteCaches(groupID) })
	resetLineRouteCaches(groupID)

	store := &lineRouteStoreStub{platform: PlatformSeedanceHub, routes: map[string]string{lineRouteW: lineRouteHH}}
	svc := lineRouteService(store)
	ctx := context.Background()
	gid := groupID

	target, ok := svc.ResolvePPVideoLineRoute(ctx, &gid, mustHubLine(t, lineRouteW))
	require.True(t, ok)
	require.Equal(t, lineRouteHH, target.Name)
	require.Equal(t, PlatformHuahuoSeedance, target.Platform)

	// A line without a route serves itself.
	_, ok = svc.ResolvePPVideoLineRoute(ctx, &gid, mustHubLine(t, lineRouteHH))
	require.False(t, ok)

	// No group means no route.
	_, ok = svc.ResolvePPVideoLineRoute(ctx, nil, mustHubLine(t, lineRouteW))
	require.False(t, ok)

	// The routes of a group are cached: the three lookups above read the store once.
	require.Equal(t, 1, store.lists)
}

func TestResolvePPVideoLineRouteIgnoresBrokenRoutes(t *testing.T) {
	const groupID = int64(910002)
	t.Cleanup(func() { resetLineRouteCaches(groupID) })
	gid := groupID
	ctx := context.Background()

	for name, routes := range map[string]map[string]string{
		"unknown target":   {lineRouteW: "no such line"},
		"self":             {lineRouteW: lineRouteW},
		"version mismatch": {lineRouteW: lineRouteW20},
	} {
		resetLineRouteCaches(groupID)
		svc := lineRouteService(&lineRouteStoreStub{platform: PlatformSeedanceHub, routes: routes})
		_, ok := svc.ResolvePPVideoLineRoute(ctx, &gid, mustHubLine(t, lineRouteW))
		require.False(t, ok, name)
	}

	// A storage error never fails a request: the line serves itself.
	resetLineRouteCaches(groupID)
	svc := lineRouteService(&lineRouteStoreStub{platform: PlatformSeedanceHub, listErr: errors.New("db down")})
	_, ok := svc.ResolvePPVideoLineRoute(ctx, &gid, mustHubLine(t, lineRouteW))
	require.False(t, ok)

	// A service without the route storage (no routing) works too.
	resetLineRouteCaches(groupID)
	_, ok = (&OpenAIGatewayService{}).ResolvePPVideoLineRoute(ctx, &gid, mustHubLine(t, lineRouteW))
	require.False(t, ok)
}

func TestSetPPVideoLineRoute(t *testing.T) {
	const groupID = int64(910003)
	t.Cleanup(func() { resetLineRouteCaches(groupID) })
	ctx := context.Background()

	t.Run("saves a route to a line the group has an account for, and resolves it at once", func(t *testing.T) {
		resetLineRouteCaches(groupID)
		store := &lineRouteStoreStub{platform: PlatformSeedanceHub}
		svc := lineRouteService(store)
		gid := groupID

		// Read once so the (empty) routes are cached; the save must drop that cache entry.
		_, ok := svc.ResolvePPVideoLineRoute(ctx, &gid, mustHubLine(t, lineRouteW))
		require.False(t, ok)

		require.NoError(t, svc.SetPPVideoLineRoute(ctx, groupID, lineRouteW, lineRouteHH))
		require.Equal(t, map[string]string{lineRouteW: lineRouteHH}, store.routes)
		target, ok := svc.ResolvePPVideoLineRoute(ctx, &gid, mustHubLine(t, lineRouteW))
		require.True(t, ok)
		require.Equal(t, lineRouteHH, target.Name)

		// An empty target, or the line itself, removes the route.
		require.NoError(t, svc.SetPPVideoLineRoute(ctx, groupID, lineRouteW, ""))
		require.Empty(t, store.routes)
		_, ok = svc.ResolvePPVideoLineRoute(ctx, &gid, mustHubLine(t, lineRouteW))
		require.False(t, ok)
		require.NoError(t, svc.SetPPVideoLineRoute(ctx, groupID, lineRouteW, lineRouteHH))
		require.NoError(t, svc.SetPPVideoLineRoute(ctx, groupID, lineRouteW, lineRouteW))
		require.Empty(t, store.routes)
	})

	t.Run("removes a route left behind by a line that is no longer in the table", func(t *testing.T) {
		resetLineRouteCaches(groupID)
		store := &lineRouteStoreStub{platform: PlatformSeedanceHub, routes: map[string]string{"a retired line": lineRouteHH}}
		require.NoError(t, lineRouteService(store).SetPPVideoLineRoute(ctx, groupID, "a retired line", ""))
		require.Empty(t, store.routes)
	})

	rejected := map[string]struct {
		platform string
		line     string
		target   string
		contains string
	}{
		"the group has no account of the target's platform": {PlatformSeedanceHub, lineRouteW, lineRouteXM, "no schedulable account"},
		"a withdrawn line cannot be a target":               {PlatformSeedanceHub, lineRouteW, lineRouteHidden, "withdrawn"},
		"another Seedance version":                          {PlatformSeedanceHub, lineRouteW, lineRouteW20, "cannot be routed"},
		"unknown target":                                    {PlatformSeedanceHub, lineRouteW, "no such line", "unknown target"},
		"unknown line":                                      {PlatformSeedanceHub, "no such line", lineRouteHH, "unknown line"},
		"not a Seedance group":                              {PlatformKling, lineRouteW, lineRouteHH, "aggregate groups"},
	}
	for name, tc := range rejected {
		t.Run("rejects: "+name, func(t *testing.T) {
			resetLineRouteCaches(groupID)
			store := &lineRouteStoreStub{platform: tc.platform}
			err := lineRouteService(store).SetPPVideoLineRoute(ctx, groupID, tc.line, tc.target)
			require.ErrorContains(t, err, tc.contains)
			require.Empty(t, store.routes)
		})
	}

	t.Run("rejects a group that does not exist", func(t *testing.T) {
		err := lineRouteService(&lineRouteStoreStub{}).SetPPVideoLineRoute(ctx, groupID, lineRouteW, lineRouteHH)
		require.ErrorIs(t, err, ErrPPVideoLineRouteGroupNotFound)
	})
}

func TestGetPPVideoLineRoutesForAdmin(t *testing.T) {
	const groupID = int64(910004)
	ctx := context.Background()
	store := &lineRouteStoreStub{platform: PlatformSeedanceHub, routes: map[string]string{lineRouteW: lineRouteHH}}
	svc := lineRouteService(store)

	lines, err := svc.GetPPVideoLineRoutesForAdmin(ctx, groupID)
	require.NoError(t, err)

	byName := make(map[string]PPVideoLineRouteLine, len(lines))
	for _, line := range lines {
		byName[line.LineName] = line
	}
	// Exactly the visible lines the group has accounts for are listed.
	require.Len(t, byName, 3)
	w, hh, w20 := byName[lineRouteW], byName[lineRouteHH], byName[lineRouteW20]
	require.Equal(t, 1, w.Accounts)
	require.Equal(t, lineRouteHH, w.TargetLineName)
	require.Equal(t, 1, w.TargetAccounts)
	require.Empty(t, hh.TargetLineName)

	// A line can only be routed to visible lines of its own version that the group serves.
	candidates := func(line PPVideoLineRouteLine) []string {
		names := make([]string, 0, len(line.Candidates))
		for _, candidate := range line.Candidates {
			names = append(names, candidate.LineName)
		}
		return names
	}
	require.Equal(t, []string{lineRouteHH}, candidates(w))
	require.Equal(t, []string{lineRouteW}, candidates(hh))
	require.Empty(t, candidates(w20))

	// A route whose target lost its accounts is still listed, flagged by TargetAccounts == 0.
	store.routes = map[string]string{lineRouteW: lineRouteXM}
	lines, err = svc.GetPPVideoLineRoutesForAdmin(ctx, groupID)
	require.NoError(t, err)
	for _, line := range lines {
		if line.LineName == lineRouteW {
			require.Equal(t, lineRouteXM, line.TargetLineName)
			require.Zero(t, line.TargetAccounts)
		}
	}
}

func TestPreparePPVideoRoutedRequest(t *testing.T) {
	t.Parallel()

	// The client asks for W pro; the request is prepared for HH: its `model` becomes the name
	// HH's pipeline accepts, and the prepared request is HH's.
	original := []byte(`{"model":"` + lineRouteW + `","prompt":"x","seconds":5,"images":["https://example.com/1.png"]}`)
	hh := mustHubLine(t, lineRouteHH)
	routed, reason, err := PreparePPVideoRoutedRequest(hh, PPVideoOperationGeneric, original, false)
	require.NoError(t, err, reason)
	require.Equal(t, PlatformHuahuoSeedance, routed.Platform)
	require.Equal(t, hh.Name, routed.Line.Name)
	require.Contains(t, string(routed.PipelineBody), hh.Model)
	require.NotContains(t, string(routed.PipelineBody), lineRouteW)

	// A request the target platform refuses (the native normalizer does not take first/last
	// frames) is reported, not turned into a client error: the caller falls back to the line the
	// client asked for.
	_, reason, err = PreparePPVideoRoutedRequest(hh, PPVideoOperationGeneric,
		[]byte(`{"model":"`+lineRouteW+`","prompt":"p","duration":5,"first_frame":"https://example.com/1.png"}`), false)
	require.Error(t, err)
	require.NotEmpty(t, reason)
}

func TestRoutedHubLineIsPricedAsTheRequestedLine(t *testing.T) {
	t.Parallel()

	groupID := int64(910005)
	price := 19.0
	svc := &OpenAIGatewayService{
		billingService: newTestBillingService(),
		resolver: newPPVideoResolverWithChannel(t, groupID, PlatformSeedanceHub, []ChannelModelPricing{
			{Platform: PlatformSeedanceHub, Models: []string{lineRouteW}, BillingMode: BillingModePerRequest, PerRequestPrice: &price},
		}),
	}
	apiKey := &APIKey{GroupID: &groupID, Group: &Group{Platform: PlatformSeedanceHub, RateMultiplier: 1, VideoRateIndependent: true, VideoRateMultiplier: 1}}
	meta := PPVideoBillingMetadata{
		// The request is run by 花火, but the client asked for (and is priced as) the 喵呜 line.
		Platform: PlatformHuahuoSeedance, Model: lineRouteW,
		RequestedDurationMilliseconds: 5000, RequestedDurationSeconds: 5, VideoCount: 1, VideoResolution: "720p",
	}

	// Without the route flag the line does not belong to the platform that ran it: refused.
	_, err := svc.CalculatePPVideoCost(context.Background(), apiKey, nil, &Account{}, meta)
	require.ErrorContains(t, err, "not a line of this group")

	meta.HubLineRouted = true
	cost, err := svc.CalculatePPVideoCost(context.Background(), apiKey, nil, &Account{}, meta)
	require.NoError(t, err)
	require.Equal(t, PPVideoBillingFormulaPerRequest, cost.BillingFormula)
	require.InDelta(t, 19, cost.TotalCost, 1e-9)

	// The flag only relaxes the platform check: a model that is no line of the group is refused.
	meta.Model = "no such line"
	_, err = svc.CalculatePPVideoCost(context.Background(), apiKey, nil, &Account{}, meta)
	require.ErrorContains(t, err, "not a line of this group")
}

// The ordinary group of a Seedance platform (not an aggregate group) can carry routes too: it
// serves the lines of its own platform, and the accounts of other platforms it holds are backups.
func TestLineRoutesInAnOrdinaryGroup(t *testing.T) {
	const groupID = int64(910006)
	t.Cleanup(func() { resetLineRouteCaches(groupID) })
	ctx := context.Background()

	// A public 喵呜 group that also holds a 花火 account as a backup.
	newStore := func() *lineRouteStoreStub { return &lineRouteStoreStub{platform: PlatformMiaowuSeedance} }

	t.Run("lists only the lines of the group's own platform, with the backup platform as a target", func(t *testing.T) {
		store := newStore()
		store.routes = map[string]string{lineRouteW: lineRouteHH}
		lines, err := lineRouteService(store).GetPPVideoLineRoutesForAdmin(ctx, groupID)
		require.NoError(t, err)

		names := make([]string, 0, len(lines))
		for _, line := range lines {
			require.Equal(t, PlatformMiaowuSeedance, line.Platform, line.LineName)
			names = append(names, line.LineName)
		}
		// The 花火 line is a backup target here, not a line the group's customers ask for.
		require.ElementsMatch(t, []string{lineRouteW, lineRouteW20}, names)
		for _, line := range lines {
			if line.LineName == lineRouteW {
				require.Equal(t, lineRouteHH, line.TargetLineName)
				require.Equal(t, 1, line.TargetAccounts)
				require.Equal(t, []string{lineRouteHH}, []string{line.Candidates[0].LineName})
			}
		}
	})

	t.Run("saves a route to the backup platform's line, but not for a line of another platform", func(t *testing.T) {
		resetLineRouteCaches(groupID)
		store := newStore()
		svc := lineRouteService(store)
		require.NoError(t, svc.SetPPVideoLineRoute(ctx, groupID, lineRouteW, lineRouteHH))
		require.Equal(t, map[string]string{lineRouteW: lineRouteHH}, store.routes)

		// A request of this group never asks for a 花火 line, so a route on it could never apply.
		err := svc.SetPPVideoLineRoute(ctx, groupID, lineRouteHH, lineRouteW)
		require.ErrorContains(t, err, "is not a line of a")
		require.Len(t, store.routes, 1)
	})

	t.Run("a route is resolved for a request of the group", func(t *testing.T) {
		resetLineRouteCaches(groupID)
		store := newStore()
		store.routes = map[string]string{lineRouteW: lineRouteHH}
		gid := groupID
		target, ok := lineRouteService(store).ResolvePPVideoLineRoute(ctx, &gid, mustHubLine(t, lineRouteW))
		require.True(t, ok)
		require.Equal(t, PlatformHuahuoSeedance, target.Platform)
	})
}

// An account with no model mapping accepts every model of its platform, so it serves every line
// of its platform (the model list of a group only shows the lines of mapped accounts, but a real
// request is selected by this rule).
func TestGroupServedLinesCountAccountsWithoutMapping(t *testing.T) {
	t.Parallel()

	served := ppVideoGroupServedLines([]Account{
		{ID: 1, Platform: PlatformMiaowuSeedance},
		lineRouteAccount(2, PlatformHuahuoSeedance, "Seedance2.5-mancan-anci"),
	})
	require.Equal(t, 1, served[lineRouteW])
	require.Equal(t, 1, served[lineRouteW20])
	// Even a line withdrawn from the site: a client that still sends its name is served.
	require.Equal(t, 1, served[lineRouteHidden])
	require.Equal(t, 1, served[lineRouteHH])
	// The 花火 account is mapped to one model only.
	require.Zero(t, served["Seedance2.5--720p--满参稳定--按秒--HH"])
	// Accounts of a platform that is not a member serve no line.
	require.Empty(t, ppVideoGroupServedLines([]Account{{ID: 3, Platform: PlatformKling}}))
}

func TestLineRoutesGlobalSwitch(t *testing.T) {
	const groupID = int64(910007)
	t.Cleanup(func() { resetLineRouteCaches(groupID) })
	ctx := context.Background()
	gid := groupID

	t.Run("a route does not apply while the switch is off, and applies once it is on", func(t *testing.T) {
		resetLineRouteCaches(groupID)
		store := &lineRouteStoreStub{platform: PlatformSeedanceHub, routes: map[string]string{lineRouteW: lineRouteHH}, switchOff: true}
		svc := lineRouteService(store)

		require.False(t, svc.PPVideoLineRoutesEnabled(ctx))
		_, ok := svc.ResolvePPVideoLineRoute(ctx, &gid, mustHubLine(t, lineRouteW))
		require.False(t, ok)
		// While the switch is off the routes themselves are not even read.
		require.Zero(t, store.lists)

		require.NoError(t, svc.SetPPVideoLineRoutesEnabled(ctx, true))
		require.True(t, svc.PPVideoLineRoutesEnabled(ctx))
		target, ok := svc.ResolvePPVideoLineRoute(ctx, &gid, mustHubLine(t, lineRouteW))
		require.True(t, ok)
		require.Equal(t, lineRouteHH, target.Name)

		require.NoError(t, svc.SetPPVideoLineRoutesEnabled(ctx, false))
		_, ok = svc.ResolvePPVideoLineRoute(ctx, &gid, mustHubLine(t, lineRouteW))
		require.False(t, ok)
	})

	t.Run("a read error, and a service without storage, mean off", func(t *testing.T) {
		resetLineRouteCaches(groupID)
		svc := lineRouteService(&lineRouteStoreStub{platform: PlatformSeedanceHub, routes: map[string]string{lineRouteW: lineRouteHH}, switchErr: errors.New("db down")})
		require.False(t, svc.PPVideoLineRoutesEnabled(ctx))
		_, ok := svc.ResolvePPVideoLineRoute(ctx, &gid, mustHubLine(t, lineRouteW))
		require.False(t, ok)

		resetLineRouteCaches(groupID)
		require.False(t, (&OpenAIGatewayService{}).PPVideoLineRoutesEnabled(ctx))
	})

	t.Run("the switch is read once per cache period, not once per request", func(t *testing.T) {
		resetLineRouteCaches(groupID)
		store := &lineRouteStoreStub{platform: PlatformSeedanceHub, switchOff: true}
		svc := lineRouteService(store)
		for i := 0; i < 5; i++ {
			require.False(t, svc.PPVideoLineRoutesEnabled(ctx))
		}
		require.Equal(t, 1, store.switchReads)
	})

	t.Run("saving a route and reading the admin view do not depend on the switch", func(t *testing.T) {
		resetLineRouteCaches(groupID)
		store := &lineRouteStoreStub{platform: PlatformSeedanceHub, switchOff: true}
		svc := lineRouteService(store)
		require.NoError(t, svc.SetPPVideoLineRoute(ctx, groupID, lineRouteW, lineRouteHH))
		lines, err := svc.GetPPVideoLineRoutesForAdmin(ctx, groupID)
		require.NoError(t, err)
		require.NotEmpty(t, lines)
	})
}
