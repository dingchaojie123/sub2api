package service

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/tidwall/gjson"
)

// This file implements the unified video request protocol (v2) served at
// POST /v2/videos/generations. A v2 body is validated once, then translated into
// the request shape that the target platform's existing normalizer already
// accepts, so the downstream pipeline (validation, billing metadata, idempotency,
// upstream calls) is reused unchanged. See docs/internal/PP_VIDEO_UNIFIED_PROTOCOL_DESIGN.md.

const (
	ppVideoUnifiedAssetRoleReference  = "reference"
	ppVideoUnifiedAssetRoleFirstFrame = "first_frame"
	ppVideoUnifiedAssetRoleLastFrame  = "last_frame"
)

// PPVideoUnifiedError describes an invalid field in a unified (v2) video request.
// Param is the JSON location of the offending field, for example "assets[2].role".
type PPVideoUnifiedError struct {
	Param   string
	Message string
}

func (e *PPVideoUnifiedError) Error() string {
	if e.Param == "" {
		return e.Message
	}
	return e.Param + ": " + e.Message
}

func ppVideoUnifiedErr(param, format string, args ...any) error {
	return &PPVideoUnifiedError{Param: param, Message: fmt.Sprintf(format, args...)}
}

// IsPPVideoUnifiedPlatform reports whether the unified (v2) protocol can be
// translated for the given video platform. Platforms are added here as their
// translation is implemented.
func IsPPVideoUnifiedPlatform(platform string) bool {
	switch strings.TrimSpace(platform) {
	case PlatformGuangying, PlatformByteDance, PlatformWan3, PlatformPixverseV6, PlatformMiniMaxH3CompShare, Platform88APIVideo, PlatformMaiTokenSeedance, PlatformXingAPISeedance, PlatformJijiuSeedance, PlatformCTMOSeedance, PlatformSeedanceHub:
		return true
	default:
		return false
	}
}

var ppVideoUnifiedTopLevelFields = map[string]struct{}{
	"model":          {},
	"prompt":         {},
	"duration":       {},
	"resolution":     {},
	"ratio":          {},
	"generate_audio": {},
	"assets":         {},
	"callback_url":   {},
	"callback_token": {},
	"seed":           {},
	"watermark":      {},
	"options":        {},
	"metadata":       {},
}

type ppVideoUnifiedAsset struct {
	Index int
	Type  string
	Role  string
	Name  string
	URL   string
}

type ppVideoUnifiedOption struct {
	Key   string
	Value gjson.Result
}

type ppVideoUnifiedRequest struct {
	model         string
	hasModel      bool
	prompt        string
	duration      *int64
	resolution    string
	ratio         string
	generateAudio *bool
	watermark     *bool
	seed          *int64
	callbackURL   string
	callbackToken string
	assets        []ppVideoUnifiedAsset
	options       []ppVideoUnifiedOption
	metadata      map[string]any
}

// TranslatePPVideoUnifiedRequest validates a unified (v2) request body and
// translates it into the request shape accepted by the existing normalizer of
// the given platform. The returned body is then handed to
// PreparePPVideoRequestBody exactly like a native request would be.
func TranslatePPVideoUnifiedRequest(platform string, body []byte) ([]byte, error) {
	platform = strings.TrimSpace(platform)
	if !IsPPVideoUnifiedPlatform(platform) {
		return nil, ppVideoUnifiedErr("", "the unified video API (v2) is not supported for this platform yet; use /v1/videos/generations")
	}
	request, err := parsePPVideoUnifiedRequest(body)
	if err != nil {
		return nil, err
	}

	var payload map[string]any
	switch platform {
	case PlatformGuangying:
		payload, err = request.toGuangying()
	case PlatformByteDance:
		payload, err = request.toByteDance()
	case PlatformWan3:
		payload, err = request.toWan()
	case PlatformPixverseV6:
		payload, err = request.toPixverse()
	case PlatformMiniMaxH3CompShare:
		payload, err = request.toCompShare()
	case Platform88APIVideo:
		payload, err = request.toGeneralModels()
	case PlatformMaiTokenSeedance:
		payload, err = request.toMaiToken()
	case PlatformXingAPISeedance:
		payload, err = request.toXingAPI()
	case PlatformJijiuSeedance:
		payload, err = request.toJijiu()
	case PlatformCTMOSeedance:
		payload, err = request.toCTMO()
	default:
		// The aggregate group never reaches this point: its requests are translated as the
		// requests of the platform that serves the line.
		return nil, ppVideoUnifiedErr("", "the unified video API (v2) is not supported for this platform yet; use /v1/videos/generations")
	}
	if err != nil {
		return nil, err
	}
	translated, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal unified video request: %w", err)
	}
	return translated, nil
}

func parsePPVideoUnifiedRequest(body []byte) (*ppVideoUnifiedRequest, error) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return nil, ppVideoUnifiedErr("", "request body must be valid JSON")
	}
	root := gjson.ParseBytes(body)
	if !root.IsObject() {
		return nil, ppVideoUnifiedErr("", "request body must be a JSON object")
	}

	var unknownField error
	root.ForEach(func(key, _ gjson.Result) bool {
		if _, ok := ppVideoUnifiedTopLevelFields[key.String()]; !ok {
			unknownField = ppVideoUnifiedErr(key.String(), "unknown field; the v2 video API does not accept it")
			return false
		}
		return true
	})
	if unknownField != nil {
		return nil, unknownField
	}

	request := &ppVideoUnifiedRequest{}
	var err error

	if request.model, request.hasModel, err = ppVideoUnifiedString(root, "model"); err != nil {
		return nil, err
	}
	if request.prompt, _, err = ppVideoUnifiedString(root, "prompt"); err != nil {
		return nil, err
	}
	if request.resolution, _, err = ppVideoUnifiedString(root, "resolution"); err != nil {
		return nil, err
	}
	if request.ratio, _, err = ppVideoUnifiedString(root, "ratio"); err != nil {
		return nil, err
	}
	if request.callbackURL, _, err = ppVideoUnifiedString(root, "callback_url"); err != nil {
		return nil, err
	}
	if request.callbackToken, _, err = ppVideoUnifiedString(root, "callback_token"); err != nil {
		return nil, err
	}
	if request.duration, err = ppVideoUnifiedInteger(root, "duration", 1); err != nil {
		return nil, err
	}
	if request.seed, err = ppVideoUnifiedInteger(root, "seed", 0); err != nil {
		return nil, err
	}
	if request.generateAudio, err = ppVideoUnifiedBool(root, "generate_audio"); err != nil {
		return nil, err
	}
	if request.watermark, err = ppVideoUnifiedBool(root, "watermark"); err != nil {
		return nil, err
	}
	if request.options, err = ppVideoUnifiedOptions(root); err != nil {
		return nil, err
	}
	if request.metadata, err = ppVideoUnifiedMetadata(root); err != nil {
		return nil, err
	}
	if request.assets, err = ppVideoUnifiedAssets(root); err != nil {
		return nil, err
	}
	return request, nil
}

// ppVideoUnifiedString reads an optional string field. A missing field or an
// explicit null both count as absent.
func ppVideoUnifiedString(parent gjson.Result, field string) (string, bool, error) {
	value := parent.Get(field)
	if !value.Exists() || value.Type == gjson.Null {
		return "", false, nil
	}
	if value.Type != gjson.String {
		return "", false, ppVideoUnifiedErr(field, "must be a string")
	}
	return strings.TrimSpace(value.String()), true, nil
}

func ppVideoUnifiedInteger(parent gjson.Result, field string, minimum int64) (*int64, error) {
	value := parent.Get(field)
	if !value.Exists() || value.Type == gjson.Null {
		return nil, nil
	}
	if value.Type != gjson.Number || value.Float() != math.Trunc(value.Float()) {
		return nil, ppVideoUnifiedErr(field, "must be an integer")
	}
	number := value.Int()
	if number < minimum {
		return nil, ppVideoUnifiedErr(field, "must be at least %d", minimum)
	}
	return &number, nil
}

func ppVideoUnifiedBool(parent gjson.Result, field string) (*bool, error) {
	value := parent.Get(field)
	if !value.Exists() || value.Type == gjson.Null {
		return nil, nil
	}
	if value.Type != gjson.True && value.Type != gjson.False {
		return nil, ppVideoUnifiedErr(field, "must be a boolean")
	}
	flag := value.Type == gjson.True
	return &flag, nil
}

func ppVideoUnifiedOptions(root gjson.Result) ([]ppVideoUnifiedOption, error) {
	value := root.Get("options")
	if !value.Exists() || value.Type == gjson.Null {
		return nil, nil
	}
	if !value.IsObject() {
		return nil, ppVideoUnifiedErr("options", "must be an object")
	}
	options := make([]ppVideoUnifiedOption, 0)
	value.ForEach(func(key, item gjson.Result) bool {
		options = append(options, ppVideoUnifiedOption{Key: key.String(), Value: item})
		return true
	})
	return options, nil
}

func ppVideoUnifiedMetadata(root gjson.Result) (map[string]any, error) {
	value := root.Get("metadata")
	if !value.Exists() || value.Type == gjson.Null {
		return nil, nil
	}
	if !value.IsObject() {
		return nil, ppVideoUnifiedErr("metadata", "must be an object")
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(value.Raw), &metadata); err != nil {
		return nil, ppVideoUnifiedErr("metadata", "must be a valid JSON object")
	}
	return metadata, nil
}

func ppVideoUnifiedAssets(root gjson.Result) ([]ppVideoUnifiedAsset, error) {
	value := root.Get("assets")
	if !value.Exists() || value.Type == gjson.Null {
		return nil, nil
	}
	if !value.IsArray() {
		return nil, ppVideoUnifiedErr("assets", "must be an array")
	}

	items := value.Array()
	assets := make([]ppVideoUnifiedAsset, 0, len(items))
	usedNames := make(map[string]struct{})
	firstFrames := 0
	lastFrames := 0
	for index, item := range items {
		prefix := fmt.Sprintf("assets[%d]", index)
		if !item.IsObject() {
			return nil, ppVideoUnifiedErr(prefix, "must be an object")
		}
		var unknownField error
		item.ForEach(func(key, _ gjson.Result) bool {
			switch key.String() {
			case "type", "url", "role", "name":
				return true
			}
			unknownField = ppVideoUnifiedErr(prefix+"."+key.String(), "unknown field")
			return false
		})
		if unknownField != nil {
			return nil, unknownField
		}

		assetType, _, err := ppVideoUnifiedString(item, "type")
		if err != nil {
			return nil, ppVideoUnifiedErr(prefix+".type", "must be a string")
		}
		assetType = strings.ToLower(assetType)
		if assetType != "image" && assetType != "video" && assetType != "audio" {
			return nil, ppVideoUnifiedErr(prefix+".type", "must be image, video, or audio")
		}

		rawURL, _, err := ppVideoUnifiedString(item, "url")
		if err != nil {
			return nil, ppVideoUnifiedErr(prefix+".url", "must be a string")
		}
		if rawURL == "" {
			return nil, ppVideoUnifiedErr(prefix+".url", "is required")
		}

		role, hasRole, err := ppVideoUnifiedString(item, "role")
		if err != nil {
			return nil, ppVideoUnifiedErr(prefix+".role", "must be a string")
		}
		if !hasRole || role == "" {
			role = ppVideoUnifiedAssetRoleReference
		}
		switch role {
		case ppVideoUnifiedAssetRoleReference:
		case ppVideoUnifiedAssetRoleFirstFrame, ppVideoUnifiedAssetRoleLastFrame:
			if assetType != "image" {
				return nil, ppVideoUnifiedErr(prefix+".role", "%q is only valid for image assets", role)
			}
			if role == ppVideoUnifiedAssetRoleFirstFrame {
				firstFrames++
			} else {
				lastFrames++
			}
		default:
			return nil, ppVideoUnifiedErr(prefix+".role", "must be first_frame, last_frame, or reference")
		}
		if firstFrames > 1 {
			return nil, ppVideoUnifiedErr(prefix+".role", "at most one first_frame image is allowed")
		}
		if lastFrames > 1 {
			return nil, ppVideoUnifiedErr(prefix+".role", "at most one last_frame image is allowed")
		}

		name, _, err := ppVideoUnifiedString(item, "name")
		if err != nil {
			return nil, ppVideoUnifiedErr(prefix+".name", "must be a string")
		}
		if name != "" {
			if _, exists := usedNames[name]; exists {
				return nil, ppVideoUnifiedErr(prefix+".name", "%q is duplicated", name)
			}
			usedNames[name] = struct{}{}
		}

		assets = append(assets, ppVideoUnifiedAsset{Index: index, Type: assetType, Role: role, Name: name, URL: rawURL})
	}
	return assets, nil
}

// assignPPVideoUnifiedAssetNames gives every unnamed asset a name made of its
// type and a per-type counter (image1, image2, audio1, ...), skipping names that
// the client already used.
func assignPPVideoUnifiedAssetNames(assets []ppVideoUnifiedAsset) {
	used := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		if asset.Name != "" {
			used[asset.Name] = struct{}{}
		}
	}
	counters := make(map[string]int)
	for index := range assets {
		if assets[index].Name != "" {
			continue
		}
		for {
			counters[assets[index].Type]++
			candidate := fmt.Sprintf("%s%d", assets[index].Type, counters[assets[index].Type])
			if _, taken := used[candidate]; taken {
				continue
			}
			assets[index].Name = candidate
			used[candidate] = struct{}{}
			break
		}
	}
}

// toGuangying translates to the Guangying request shape: top-level prompt,
// duration, resolution and ratio, plus an assets array whose names the platform
// keeps and exposes to the prompt as @name.
func (r *ppVideoUnifiedRequest) toGuangying() (map[string]any, error) {
	if r.generateAudio != nil {
		return nil, ppVideoUnifiedErr("generate_audio", "is not supported by this model")
	}
	if r.seed != nil {
		return nil, ppVideoUnifiedErr("seed", "is not supported by this model")
	}
	if r.watermark != nil {
		return nil, ppVideoUnifiedErr("watermark", "is not supported by this model")
	}
	if r.callbackURL != "" {
		return nil, ppVideoUnifiedErr("callback_url", "is not supported by this model")
	}
	if r.callbackToken != "" {
		return nil, ppVideoUnifiedErr("callback_token", "is not supported by this model")
	}
	if r.prompt == "" {
		return nil, ppVideoUnifiedErr("prompt", "is required for this model")
	}

	nativeFace := false
	hasNativeFace := false
	for _, option := range r.options {
		if option.Key != "native_face" {
			return nil, ppVideoUnifiedErr("options."+option.Key, "is not a supported option for this model")
		}
		if option.Value.Type != gjson.True && option.Value.Type != gjson.False {
			return nil, ppVideoUnifiedErr("options.native_face", "must be a boolean")
		}
		nativeFace = option.Value.Type == gjson.True
		hasNativeFace = true
	}

	// This platform's asset objects carry no first/last-frame marker, so frame
	// images are submitted as ordinary image assets in the order they were sent,
	// exactly like the native request does with start_frame_url / end_frame_url.
	// The role is accepted so the same request body works across models; it does
	// not change how the upstream treats the image.
	assets := append([]ppVideoUnifiedAsset(nil), r.assets...)
	assignPPVideoUnifiedAssetNames(assets)

	payload := map[string]any{"prompt": r.prompt}
	if r.hasModel {
		payload["model"] = r.model
	}
	if r.duration != nil {
		payload["duration"] = *r.duration
	}
	if r.resolution != "" {
		payload["resolution"] = r.resolution
	}
	if r.ratio != "" {
		payload["ratio"] = r.ratio
	}
	if len(assets) > 0 {
		items := make([]any, 0, len(assets))
		for _, asset := range assets {
			items = append(items, map[string]any{"type": asset.Type, "name": asset.Name, "url": asset.URL})
		}
		payload["assets"] = items
	}
	if len(r.metadata) > 0 {
		payload["metadata"] = r.metadata
	}
	if hasNativeFace {
		payload["native_face"] = nativeFace
	}
	return payload, nil
}

// toByteDance translates to the documented native doubao shape: input.content
// items with roles, plus a parameters object. Asset names are validated but not
// forwarded because this platform has no notion of named references.
func (r *ppVideoUnifiedRequest) toByteDance() (map[string]any, error) {
	if r.callbackToken != "" {
		return nil, ppVideoUnifiedErr("callback_token", "is not supported by this model")
	}
	if len(r.metadata) > 0 {
		return nil, ppVideoUnifiedErr("metadata", "is not supported by this model")
	}

	content := make([]any, 0, len(r.assets)+1)
	if r.prompt != "" {
		content = append(content, map[string]any{"type": "text", "text": r.prompt})
	}
	for _, asset := range r.assets {
		switch asset.Type {
		case "image":
			role := "reference_image"
			switch asset.Role {
			case ppVideoUnifiedAssetRoleFirstFrame:
				role = "first_frame"
			case ppVideoUnifiedAssetRoleLastFrame:
				role = "last_frame"
			}
			content = append(content, map[string]any{
				"type":      "image_url",
				"image_url": map[string]any{"url": asset.URL},
				"role":      role,
			})
		case "video":
			content = append(content, map[string]any{
				"type":      "video_url",
				"video_url": map[string]any{"url": asset.URL},
				"role":      "reference_video",
			})
		case "audio":
			content = append(content, map[string]any{
				"type":      "audio_url",
				"audio_url": map[string]any{"url": asset.URL},
				"role":      "reference_audio",
			})
		}
	}
	if len(content) == 0 {
		return nil, ppVideoUnifiedErr("prompt", "prompt or at least one asset is required")
	}

	parameters := make(map[string]any)
	if r.duration != nil {
		parameters["duration"] = *r.duration
	}
	if r.resolution != "" {
		parameters["resolution"] = r.resolution
	}
	if r.ratio != "" {
		parameters["ratio"] = r.ratio
	}
	if r.generateAudio != nil {
		parameters["generate_audio"] = *r.generateAudio
	}
	if r.seed != nil {
		parameters["seed"] = *r.seed
	}
	if r.watermark != nil {
		parameters["watermark"] = *r.watermark
	}
	if r.callbackURL != "" {
		parameters["callback_url"] = r.callbackURL
	}
	for _, option := range r.options {
		switch option.Key {
		case "camera_fixed":
			if option.Value.Type != gjson.True && option.Value.Type != gjson.False {
				return nil, ppVideoUnifiedErr("options.camera_fixed", "must be a boolean")
			}
			parameters["camera_fixed"] = option.Value.Type == gjson.True
		case "execution_expires_after", "seedance_tools", "omni_reference_task_type":
			parameters[option.Key] = option.Value.Value()
		default:
			return nil, ppVideoUnifiedErr("options."+option.Key, "is not a supported option for this model")
		}
	}

	payload := map[string]any{
		"input":      map[string]any{"content": content},
		"parameters": parameters,
	}
	if r.hasModel {
		payload["model"] = r.model
	}
	return payload, nil
}
