package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

// Line routes of a Seedance aggregate group.
//
// An operator can point one line of a group at another line ("route"): from then on the requests
// of that group that ask for the line are run by the target line's upstream platform, while the
// client keeps sending, being billed for and seeing the line it asked for. Only the upstream that
// runs the request changes. Routes are per group, because a customer's exclusive group has its
// own accounts and its own channel prices; a route is only allowed when the group has schedulable
// accounts that serve the target line, so a switch can never leave a group without an upstream.
//
// A route is one hop: a request for A is served by A's target B, even when B has a route of its
// own. Tasks that were already submitted stay on the account that took them; only new
// submissions follow the route.

var errPPVideoLineRouteRepositoryMissing = ErrPPVideoTaskRepositoryMissing

// ErrPPVideoLineRouteGroupNotFound is returned when the group of a route does not exist.
var ErrPPVideoLineRouteGroupNotFound = infraerrors.NotFound("PP_VIDEO_LINE_ROUTE_GROUP_NOT_FOUND", "group not found")

func ppVideoLineRouteInvalid(format string, args ...any) error {
	return infraerrors.BadRequest("PP_VIDEO_LINE_ROUTE_INVALID", fmt.Sprintf(format, args...))
}

// PPVideoLineRouteRepository stores the routes. It is an optional capability of the usage billing
// repository, like the task queries, so the existing repository interfaces stay unchanged.
type PPVideoLineRouteRepository interface {
	// ListPPVideoLineRoutes returns the routes of one group: line name -> target line name.
	ListPPVideoLineRoutes(ctx context.Context, groupID int64) (map[string]string, error)
	// SetPPVideoLineRoute creates or replaces the route of one line of a group.
	SetPPVideoLineRoute(ctx context.Context, groupID int64, lineName, targetLineName string) error
	// DeletePPVideoLineRoute removes the route of one line of a group; a missing route is not an error.
	DeletePPVideoLineRoute(ctx context.Context, groupID int64, lineName string) error
	// GetPPVideoLineRouteGroupPlatform returns the platform of a group, or ErrPPVideoLineRouteGroupNotFound.
	GetPPVideoLineRouteGroupPlatform(ctx context.Context, groupID int64) (string, error)
	// GetPPVideoLineRoutesEnabled reads the global switch of line routes; absent means off.
	GetPPVideoLineRoutesEnabled(ctx context.Context) (bool, error)
	// SetPPVideoLineRoutesEnabled writes the global switch of line routes.
	SetPPVideoLineRoutesEnabled(ctx context.Context, enabled bool) error
}

func (s *OpenAIGatewayService) ppVideoLineRouteRepo() (PPVideoLineRouteRepository, error) {
	if s == nil || s.usageBillingRepo == nil {
		return nil, errPPVideoLineRouteRepositoryMissing
	}
	repo, ok := s.usageBillingRepo.(PPVideoLineRouteRepository)
	if !ok || repo == nil {
		return nil, errPPVideoLineRouteRepositoryMissing
	}
	return repo, nil
}

// The routes of a group are read on every submission, so they are cached for a few seconds. A
// write through this service drops the entry of its group at once; another instance sees the
// change after the TTL.
const ppVideoLineRouteCacheTTL = 5 * time.Second

type ppVideoLineRouteCacheEntry struct {
	routes  map[string]string // seedanceHubKey(line name) -> target line name
	expires time.Time
}

var ppVideoLineRouteCache = struct {
	sync.RWMutex
	entries map[int64]ppVideoLineRouteCacheEntry
}{entries: make(map[int64]ppVideoLineRouteCacheEntry)}

func invalidatePPVideoLineRouteCache(groupID int64) {
	ppVideoLineRouteCache.Lock()
	delete(ppVideoLineRouteCache.entries, groupID)
	ppVideoLineRouteCache.Unlock()
}

// The global switch of line routes. It is OFF until an operator turns it on (the dialog of a
// group's line routes has the switch); while it is off no request is routed, whatever routes are
// saved, and the video submission flow is the one it was before line routes existed. It is read
// through a short cache like the routes. A missing storage, a missing row and a read error all mean
// off: a problem with the switch must never turn routing on.
const ppVideoLineRoutesSwitchCacheTTL = 5 * time.Second

var ppVideoLineRoutesSwitchCache = struct {
	sync.RWMutex
	enabled bool
	expires time.Time
}{}

func invalidatePPVideoLineRoutesSwitchCache() {
	ppVideoLineRoutesSwitchCache.Lock()
	ppVideoLineRoutesSwitchCache.expires = time.Time{}
	ppVideoLineRoutesSwitchCache.Unlock()
}

// PPVideoLineRoutesEnabled reports whether line routes are switched on.
func (s *OpenAIGatewayService) PPVideoLineRoutesEnabled(ctx context.Context) bool {
	now := time.Now()
	ppVideoLineRoutesSwitchCache.RLock()
	enabled, expires := ppVideoLineRoutesSwitchCache.enabled, ppVideoLineRoutesSwitchCache.expires
	ppVideoLineRoutesSwitchCache.RUnlock()
	if now.Before(expires) {
		return enabled
	}
	repo, err := s.ppVideoLineRouteRepo()
	if err != nil {
		return false
	}
	enabled, err = repo.GetPPVideoLineRoutesEnabled(ctx)
	if err != nil {
		logger.L().Warn("pp_video.line_routes_switch_read_failed", zap.Error(err))
		enabled = false
	}
	ppVideoLineRoutesSwitchCache.Lock()
	ppVideoLineRoutesSwitchCache.enabled = enabled
	ppVideoLineRoutesSwitchCache.expires = now.Add(ppVideoLineRoutesSwitchCacheTTL)
	ppVideoLineRoutesSwitchCache.Unlock()
	return enabled
}

// SetPPVideoLineRoutesEnabled turns line routes on or off for every group.
func (s *OpenAIGatewayService) SetPPVideoLineRoutesEnabled(ctx context.Context, enabled bool) error {
	repo, err := s.ppVideoLineRouteRepo()
	if err != nil {
		return err
	}
	if err := repo.SetPPVideoLineRoutesEnabled(ctx, enabled); err != nil {
		return err
	}
	invalidatePPVideoLineRoutesSwitchCache()
	logger.L().Info("pp_video.line_routes_switch_set", zap.Bool("enabled", enabled))
	return nil
}

func (s *OpenAIGatewayService) ppVideoLineRoutes(ctx context.Context, groupID int64) (map[string]string, error) {
	now := time.Now()
	ppVideoLineRouteCache.RLock()
	entry, ok := ppVideoLineRouteCache.entries[groupID]
	ppVideoLineRouteCache.RUnlock()
	if ok && now.Before(entry.expires) {
		return entry.routes, nil
	}
	repo, err := s.ppVideoLineRouteRepo()
	if err != nil {
		return nil, err
	}
	stored, err := repo.ListPPVideoLineRoutes(ctx, groupID)
	if err != nil {
		return nil, err
	}
	routes := make(map[string]string, len(stored))
	for line, target := range stored {
		routes[seedanceHubKey(line)] = target
	}
	ppVideoLineRouteCache.Lock()
	ppVideoLineRouteCache.entries[groupID] = ppVideoLineRouteCacheEntry{routes: routes, expires: now.Add(ppVideoLineRouteCacheTTL)}
	ppVideoLineRouteCache.Unlock()
	return routes, nil
}

// ResolvePPVideoLineRoute returns the line that serves `line` for the requests of a group, and
// true, when the group has a valid route for it. A missing route, an unknown or incompatible
// target and every storage error mean "the line serves itself": a routing problem must never
// make a request fail that would have worked without routing.
func (s *OpenAIGatewayService) ResolvePPVideoLineRoute(ctx context.Context, groupID *int64, line SeedanceHubLine) (SeedanceHubLine, bool) {
	if s == nil || groupID == nil || *groupID <= 0 {
		return SeedanceHubLine{}, false
	}
	if !s.PPVideoLineRoutesEnabled(ctx) {
		return SeedanceHubLine{}, false
	}
	routes, err := s.ppVideoLineRoutes(ctx, *groupID)
	if err != nil {
		if !errors.Is(err, errPPVideoLineRouteRepositoryMissing) {
			logger.L().Warn("pp_video.line_route_lookup_failed",
				zap.Int64("group_id", *groupID), zap.String("line", line.Name), zap.Error(err))
		}
		return SeedanceHubLine{}, false
	}
	targetName, ok := routes[seedanceHubKey(line.Name)]
	if !ok {
		return SeedanceHubLine{}, false
	}
	target, ok := SeedanceHubLineByName(targetName)
	if !ok || target.Name == line.Name || target.Version != line.Version {
		logger.L().Warn("pp_video.line_route_ignored",
			zap.Int64("group_id", *groupID), zap.String("line", line.Name), zap.String("target", targetName))
		return SeedanceHubLine{}, false
	}
	return target, true
}

// PPVideoRoutedRequest is a client request prepared for the line it is routed to.
type PPVideoRoutedRequest struct {
	Line         SeedanceHubLine
	Platform     string
	PipelineBody []byte
	Body         []byte
	Public       PPVideoPublicRequest
}

// PreparePPVideoRoutedRequest prepares the client's original request body for the target line of
// a route: the `model` is rewritten to the name the target platform accepts, a v2 (unified)
// request is translated for that platform, and the platform's own request preparation and
// billing metadata validation run. A non-nil error (with a short reason) means the target cannot
// take this particular request; the caller then serves the line the client asked for instead.
func PreparePPVideoRoutedRequest(target SeedanceHubLine, operation PPVideoOperation, originalBody []byte, unified bool) (*PPVideoRoutedRequest, string, error) {
	pipelineBody := ReplaceModelInBody(originalBody, target.Model)
	if unified {
		translated, err := TranslatePPVideoUnifiedRequest(target.Platform, pipelineBody)
		if err != nil {
			return nil, "translate", err
		}
		pipelineBody = translated
	}
	body, public, err := PreparePPVideoRequestBody(target.Platform, operation, pipelineBody)
	if err != nil {
		return nil, "prepare", err
	}
	if meta := PPVideoBillingMetadataFromRequest(target.Platform, body); meta.ValidationError != nil {
		return nil, "billing_metadata", meta.ValidationError
	}
	return &PPVideoRoutedRequest{Line: target, Platform: target.Platform, PipelineBody: pipelineBody, Body: body, Public: public}, "", nil
}

// PPVideoLineRouteLine is one line of a group in the admin view.
type PPVideoLineRouteLine struct {
	LineName string `json:"line_name"`
	Platform string `json:"platform"`
	Version  string `json:"version"`
	Billing  string `json:"billing"`
	// Hidden lines are withdrawn from the site; they are listed only because they carry a route.
	Hidden bool `json:"hidden"`
	// Accounts is how many schedulable accounts of the group serve the line itself.
	Accounts int `json:"accounts"`
	// TargetLineName is the line that serves this one now; empty means the line serves itself.
	TargetLineName string `json:"target_line_name"`
	// TargetAccounts is how many schedulable accounts of the group serve the current target. It
	// is 0 when the route is broken (the target lost its accounts); such a route is skipped at
	// request time only if the target does not exist, so a broken one is shown to the operator.
	TargetAccounts int `json:"target_accounts"`
	// Candidates are the lines the operator can route this line to: visible lines of the same
	// version that the group has schedulable accounts for.
	Candidates []PPVideoLineRouteCandidate `json:"candidates"`
}

// PPVideoLineRouteCandidate is a line a route can point at.
type PPVideoLineRouteCandidate struct {
	LineName string `json:"line_name"`
	Platform string `json:"platform"`
	Accounts int    `json:"accounts"`
}

// ppVideoGroupServedLines counts, per line name, the schedulable accounts that serve the line. An
// account serves a line when it belongs to the line's platform and the request pipeline would
// accept the line's model for it: the rule a real request is selected by, so an account with no
// model mapping (which accepts every model of its platform) counts for every line of its platform.
func ppVideoGroupServedLines(accounts []Account) map[string]int {
	served := make(map[string]int)
	lines := SeedanceHubLines()
	for i := range accounts {
		acc := &accounts[i]
		if !IsSeedanceHubMemberPlatform(acc.Platform) {
			continue
		}
		for _, line := range lines {
			if line.Platform == acc.Platform && isPPVideoAccountEligibleForModel(acc, line.Model) {
				served[line.Name]++
			}
		}
	}
	return served
}

func (s *OpenAIGatewayService) ppVideoLineRouteGroupAccounts(ctx context.Context, groupID int64) ([]Account, error) {
	if s == nil || s.accountRepo == nil {
		return nil, ErrNoAvailableAccounts
	}
	return s.accountRepo.ListSchedulableByGroupID(ctx, groupID)
}

// ppVideoLineRouteGroupPlatform returns the platform of a group that can carry line routes: a
// Seedance aggregate group, or the ordinary group of a Seedance platform (see ppVideoRouteSourceLine
// in the handler: the line of a request there is found from the platform and the requested model).
func (s *OpenAIGatewayService) ppVideoLineRouteGroupPlatform(ctx context.Context, repo PPVideoLineRouteRepository, groupID int64) (string, error) {
	if groupID <= 0 {
		return "", ppVideoLineRouteInvalid("group_id is required")
	}
	platform, err := repo.GetPPVideoLineRouteGroupPlatform(ctx, groupID)
	if err != nil {
		return "", err
	}
	platform = strings.TrimSpace(platform)
	if platform != PlatformSeedanceHub && !IsSeedanceHubMemberPlatform(platform) {
		return "", ppVideoLineRouteInvalid("line routes are only available for Seedance aggregate groups and Seedance platform groups")
	}
	return platform, nil
}

// ppVideoLineRouteGroupOwnsLine reports whether requests of a group can ask for the line: any
// line in an aggregate group, only the lines of the group's own platform in an ordinary group.
func ppVideoLineRouteGroupOwnsLine(groupPlatform string, line SeedanceHubLine) bool {
	return groupPlatform == PlatformSeedanceHub || line.Platform == groupPlatform
}

// GetPPVideoLineRoutesForAdmin lists the lines of a group with their routes and the targets the
// operator can choose.
func (s *OpenAIGatewayService) GetPPVideoLineRoutesForAdmin(ctx context.Context, groupID int64) ([]PPVideoLineRouteLine, error) {
	repo, err := s.ppVideoLineRouteRepo()
	if err != nil {
		return nil, err
	}
	groupPlatform, err := s.ppVideoLineRouteGroupPlatform(ctx, repo, groupID)
	if err != nil {
		return nil, err
	}
	stored, err := repo.ListPPVideoLineRoutes(ctx, groupID)
	if err != nil {
		return nil, err
	}
	accounts, err := s.ppVideoLineRouteGroupAccounts(ctx, groupID)
	if err != nil && !errors.Is(err, ErrNoAvailableAccounts) {
		return nil, err
	}
	served := ppVideoGroupServedLines(accounts)
	routes := make(map[string]string, len(stored))
	for line, target := range stored {
		routes[seedanceHubKey(line)] = target
	}

	all := SeedanceHubLines()
	lines := make([]PPVideoLineRouteLine, 0, len(served)+len(routes))
	for _, line := range all {
		target := routes[seedanceHubKey(line.Name)]
		// A line is listed when the group serves it (visible lines only) or it already has a route
		// (so the operator can always see and remove it). An ordinary group lists only the lines of
		// its own platform as lines to route: the accounts of other platforms it holds are backups,
		// they are not lines its customers ask for.
		if !ppVideoLineRouteGroupOwnsLine(groupPlatform, line) {
			continue
		}
		if (line.Hidden || served[line.Name] == 0) && target == "" {
			continue
		}
		item := PPVideoLineRouteLine{
			LineName:       line.Name,
			Platform:       line.Platform,
			Version:        line.Version,
			Billing:        string(line.Billing),
			Hidden:         line.Hidden,
			Accounts:       served[line.Name],
			TargetLineName: target,
			Candidates:     []PPVideoLineRouteCandidate{},
		}
		if target != "" {
			if targetLine, ok := SeedanceHubLineByName(target); ok {
				item.TargetLineName = targetLine.Name
				item.TargetAccounts = served[targetLine.Name]
			}
		}
		for _, candidate := range all {
			if candidate.Hidden || candidate.Name == line.Name || candidate.Version != line.Version || served[candidate.Name] == 0 {
				continue
			}
			item.Candidates = append(item.Candidates, PPVideoLineRouteCandidate{
				LineName: candidate.Name, Platform: candidate.Platform, Accounts: served[candidate.Name],
			})
		}
		lines = append(lines, item)
	}
	return lines, nil
}

// SetPPVideoLineRoute points a line of a group at another line. An empty target removes the
// route. The target must be a visible line of the same version that the group has schedulable
// accounts for.
func (s *OpenAIGatewayService) SetPPVideoLineRoute(ctx context.Context, groupID int64, lineName, targetLineName string) error {
	repo, err := s.ppVideoLineRouteRepo()
	if err != nil {
		return err
	}
	groupPlatform, err := s.ppVideoLineRouteGroupPlatform(ctx, repo, groupID)
	if err != nil {
		return err
	}
	line, known := SeedanceHubLineByName(lineName)
	if strings.TrimSpace(targetLineName) == "" || (known && seedanceHubKey(targetLineName) == seedanceHubKey(line.Name)) {
		// Removing a route needs no valid line: a route left behind by a line that has since left
		// the line table can still be removed.
		name := strings.TrimSpace(lineName)
		if known {
			name = line.Name
		}
		if err := repo.DeletePPVideoLineRoute(ctx, groupID, name); err != nil {
			return err
		}
		invalidatePPVideoLineRouteCache(groupID)
		return nil
	}
	if !known {
		return ppVideoLineRouteInvalid("unknown line %q", strings.TrimSpace(lineName))
	}
	if !ppVideoLineRouteGroupOwnsLine(groupPlatform, line) {
		return ppVideoLineRouteInvalid("line %q is not a line of a %s group", line.Name, groupPlatform)
	}
	target, ok := SeedanceHubLineByName(targetLineName)
	if !ok {
		return ppVideoLineRouteInvalid("unknown target line %q", strings.TrimSpace(targetLineName))
	}
	if target.Hidden {
		return ppVideoLineRouteInvalid("target line %q is withdrawn from the site", target.Name)
	}
	if target.Version != line.Version {
		return ppVideoLineRouteInvalid("a Seedance %s line cannot be routed to a Seedance %s line", line.Version, target.Version)
	}
	accounts, err := s.ppVideoLineRouteGroupAccounts(ctx, groupID)
	if err != nil && !errors.Is(err, ErrNoAvailableAccounts) {
		return err
	}
	if ppVideoGroupServedLines(accounts)[target.Name] == 0 {
		return ppVideoLineRouteInvalid("the group has no schedulable account that serves %q: add an account of that platform to the group and map the model first", target.Name)
	}
	if err := repo.SetPPVideoLineRoute(ctx, groupID, line.Name, target.Name); err != nil {
		return err
	}
	invalidatePPVideoLineRouteCache(groupID)
	logger.L().Info("pp_video.line_route_set",
		zap.Int64("group_id", groupID), zap.String("line", line.Name), zap.String("target", target.Name))
	return nil
}
