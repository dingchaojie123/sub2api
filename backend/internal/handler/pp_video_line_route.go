package handler

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Line routes of Seedance aggregate groups (see service/pp_video_line_route.go).
//
//	GET /api/v1/admin/groups/:id/line-routes   lines of the group, their routes and route targets
//	PUT /api/v1/admin/groups/:id/line-routes   {"line_name": "...", "target_line_name": "..."}
//	                                           an empty target_line_name removes the route
//	GET /api/v1/admin/groups/line-routes-switch   the global switch of line routes: {"enabled": bool}
//	PUT /api/v1/admin/groups/line-routes-switch   {"enabled": bool}; the switch is OFF until turned on

// routePPVideoSubmission prepares the client's request for the line its group routes `line` to.
// It returns nil, and the request is served by `line` itself, when the group has no route for the
// line or when the target cannot take this particular request (the request is invalid for the
// target's platform): a route must never turn a request that works into an error.
//
// The request is always validated against the line the client asked for first (by the caller), so
// the client gets the same errors whatever the route is.
func (h *OpenAIGatewayHandler) routePPVideoSubmission(
	c *gin.Context,
	reqLog *zap.Logger,
	groupID *int64,
	line service.SeedanceHubLine,
	operation service.PPVideoOperation,
	originalBody []byte,
	unified bool,
) *service.PPVideoRoutedRequest {
	target, routed := h.gatewayService.ResolvePPVideoLineRoute(c.Request.Context(), groupID, line)
	if !routed {
		return nil
	}
	prepared, reason, err := service.PreparePPVideoRoutedRequest(target, operation, originalBody, unified)
	if err != nil {
		fields := []zap.Field{zap.String("line", line.Name), zap.String("target", target.Name), zap.String("reason", reason), zap.Error(err)}
		if groupID != nil {
			fields = append(fields, zap.Int64("group_id", *groupID))
		}
		reqLog.Info("pp_video.line_route_skipped", fields...)
		return nil
	}
	return prepared
}

// ppVideoRoutedAccountCheck runs the checks of a submission that depend on the selected account
// (its model mapping may refuse the model) for a request routed to another line. A non-nil error
// means the request would be refused for this account; the caller serves the line the client asked
// for instead of returning the error.
func ppVideoRoutedAccountCheck(account *service.Account, operation service.PPVideoOperation, source []byte) (string, error) {
	prepared, _, err := service.PreparePPVideoRequestBodyForAccount(account, operation, source)
	if err != nil {
		return "prepare_for_account", err
	}
	if meta := service.PPVideoBillingMetadataFromRequest(account.Platform, prepared); meta.ValidationError != nil {
		return "billing_metadata", meta.ValidationError
	}
	return "", nil
}

// ppVideoRouteSourceLine finds the line an ordinary group of a Seedance platform serves for a
// request, from the platform's own name of the requested model (the model of the request as the
// platform's pipeline prepared it). It returns nil when the model is no line of the platform.
func ppVideoRouteSourceLine(groupPlatform string, public service.PPVideoPublicRequest, preparedBody []byte) *service.SeedanceHubLine {
	model := strings.TrimSpace(public.UpstreamModel)
	if model == "" {
		model = strings.TrimSpace(service.PPVideoModelFromBody(preparedBody))
	}
	if model == "" {
		return nil
	}
	line, ok := service.SeedanceHubLineFor(groupPlatform, model)
	if !ok {
		return nil
	}
	return &line
}

type ppVideoLineRouteRequest struct {
	LineName       string `json:"line_name"`
	TargetLineName string `json:"target_line_name"`
}

// AdminGetGroupLineRoutes lists the lines of a Seedance aggregate group with their routes.
func (h *OpenAIGatewayHandler) AdminGetGroupLineRoutes(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || groupID <= 0 {
		response.BadRequest(c, "Invalid group id")
		return
	}
	lines, err := h.gatewayService.GetPPVideoLineRoutesForAdmin(c.Request.Context(), groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"group_id": groupID, "lines": lines})
}

// AdminSetGroupLineRoute points a line of a group at another line, or removes the route.
func (h *OpenAIGatewayHandler) AdminSetGroupLineRoute(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || groupID <= 0 {
		response.BadRequest(c, "Invalid group id")
		return
	}
	var req ppVideoLineRouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if err := h.gatewayService.SetPPVideoLineRoute(c.Request.Context(), groupID, req.LineName, req.TargetLineName); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	lines, err := h.gatewayService.GetPPVideoLineRoutesForAdmin(c.Request.Context(), groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"group_id": groupID, "lines": lines})
}

type ppVideoLineRoutesSwitchRequest struct {
	Enabled *bool `json:"enabled"`
}

// AdminGetLineRoutesSwitch returns the global switch of line routes.
func (h *OpenAIGatewayHandler) AdminGetLineRoutesSwitch(c *gin.Context) {
	response.Success(c, gin.H{"enabled": h.gatewayService.PPVideoLineRoutesEnabled(c.Request.Context())})
}

// AdminSetLineRoutesSwitch turns line routes on or off for every group.
func (h *OpenAIGatewayHandler) AdminSetLineRoutesSwitch(c *gin.Context) {
	var req ppVideoLineRoutesSwitchRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		response.BadRequest(c, "enabled is required")
		return
	}
	if err := h.gatewayService.SetPPVideoLineRoutesEnabled(c.Request.Context(), *req.Enabled); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"enabled": *req.Enabled})
}
