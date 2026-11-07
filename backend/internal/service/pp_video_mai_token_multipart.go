package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/tidwall/gjson"
)

type MaiTokenUpload struct {
	FieldName   string
	FileName    string
	ContentType string
	TempPath    string
	Size        int64
	SHA256      string
}

type MaiTokenMultipartSubmission struct {
	Uploads []MaiTokenUpload

	cleanupOnce sync.Once
	cleanupErr  error
}

type PPVideoSubmissionBody struct {
	JSON      []byte
	Multipart *MaiTokenMultipartSubmission
}

func PrepareMaiTokenMultipartSubmission(req *http.Request) (_ *PPVideoSubmissionBody, err error) {
	if req == nil {
		return nil, fmt.Errorf("Mai Token Seedance multipart request is required")
	}
	mediaType, _, parseErr := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if parseErr != nil || !strings.EqualFold(mediaType, "multipart/form-data") {
		return nil, fmt.Errorf("Mai Token Seedance request must use multipart/form-data")
	}
	reader, err := req.MultipartReader()
	if err != nil {
		return nil, fmt.Errorf("parse Mai Token Seedance multipart request: %w", err)
	}

	submission := &PPVideoSubmissionBody{Multipart: &MaiTokenMultipartSubmission{}}
	defer func() {
		if err != nil {
			_ = submission.Cleanup()
		}
	}()
	fields := make(map[string][]string)
	for {
		part, nextErr := reader.NextPart()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return nil, fmt.Errorf("read Mai Token Seedance multipart request: %w", nextErr)
		}
		fieldName := strings.TrimSpace(part.FormName())
		if fieldName == "" {
			_ = part.Close()
			return nil, fmt.Errorf("Mai Token Seedance multipart field name is required")
		}
		if part.FileName() == "" {
			value, readErr := io.ReadAll(part)
			_ = part.Close()
			if readErr != nil {
				return nil, fmt.Errorf("read Mai Token Seedance field %s: %w", fieldName, readErr)
			}
			fields[fieldName] = append(fields[fieldName], string(value))
			continue
		}
		if !maiTokenUploadFieldAllowed(fieldName) {
			_ = part.Close()
			return nil, fmt.Errorf("Mai Token Seedance does not support multipart file field %s", fieldName)
		}
		upload, spoolErr := spoolMaiTokenUpload(part, fieldName)
		_ = part.Close()
		if spoolErr != nil {
			return nil, spoolErr
		}
		submission.Multipart.Uploads = append(submission.Multipart.Uploads, upload)
	}

	rawJSON, err := maiTokenMultipartFieldsJSON(fields)
	if err != nil {
		return nil, err
	}
	canonical, _, err := PreparePPVideoRequestBody(PlatformMaiTokenSeedance, PPVideoOperationGeneric, rawJSON)
	if err != nil {
		return nil, err
	}
	submission.JSON = canonical
	if err = validateMaiTokenMultipartUploads(submission); err != nil {
		return nil, err
	}
	return submission, nil
}

func maiTokenUploadFieldAllowed(fieldName string) bool {
	switch fieldName {
	case "referenceImages", "first_image", "last_image":
		return true
	default:
		return false
	}
}

func spoolMaiTokenUpload(part *multipart.Part, fieldName string) (_ MaiTokenUpload, err error) {
	fileName := strings.TrimSpace(filepath.Base(part.FileName()))
	if fileName == "" || fileName == "." {
		return MaiTokenUpload{}, fmt.Errorf("Mai Token Seedance %s filename is required", fieldName)
	}
	contentType := strings.TrimSpace(part.Header.Get("Content-Type"))
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		return MaiTokenUpload{}, fmt.Errorf("Mai Token Seedance %s must be an image file", fieldName)
	}
	temp, err := os.CreateTemp("", "sub2api-mai-token-*")
	if err != nil {
		return MaiTokenUpload{}, fmt.Errorf("create Mai Token Seedance upload spool: %w", err)
	}
	tempPath := temp.Name()
	defer func() {
		if closeErr := temp.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(tempPath)
		}
	}()
	digest := sha256.New()
	size, err := io.Copy(io.MultiWriter(temp, digest), part)
	if err != nil {
		return MaiTokenUpload{}, fmt.Errorf("spool Mai Token Seedance %s: %w", fieldName, err)
	}
	if size <= 0 {
		return MaiTokenUpload{}, fmt.Errorf("Mai Token Seedance %s file must not be empty", fieldName)
	}
	return MaiTokenUpload{
		FieldName:   fieldName,
		FileName:    fileName,
		ContentType: contentType,
		TempPath:    tempPath,
		Size:        size,
		SHA256:      hex.EncodeToString(digest.Sum(nil)),
	}, nil
}

func maiTokenMultipartFieldsJSON(fields map[string][]string) ([]byte, error) {
	payload := make(map[string]any, len(fields))
	for name, values := range fields {
		if len(values) == 0 {
			continue
		}
		if maiTokenMultipartArrayField(name) {
			items := make([]string, 0, len(values))
			for _, raw := range values {
				trimmed := strings.TrimSpace(raw)
				if strings.HasPrefix(trimmed, "[") {
					var decoded []string
					if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
						return nil, fmt.Errorf("Mai Token Seedance %s must contain URL strings", name)
					}
					items = append(items, decoded...)
				} else {
					items = append(items, raw)
				}
			}
			payload[name] = items
			continue
		}
		if len(values) != 1 {
			return nil, fmt.Errorf("Mai Token Seedance multipart field %s must be provided once", name)
		}
		raw := values[0]
		switch name {
		case "duration", "seconds":
			var number json.Number
			if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &number); err == nil {
				payload[name] = number
			} else {
				payload[name] = raw
			}
		case "generate_audio", "generateAudio", "watermark":
			value, boolErr := strconv.ParseBool(strings.TrimSpace(raw))
			if boolErr == nil {
				payload[name] = value
			} else {
				payload[name] = raw
			}
		case "references", "content", "options":
			var decoded any
			if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
				return nil, fmt.Errorf("Mai Token Seedance %s must be valid JSON", name)
			}
			payload[name] = decoded
		default:
			payload[name] = raw
		}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal Mai Token Seedance multipart fields: %w", err)
	}
	return encoded, nil
}

func maiTokenMultipartArrayField(name string) bool {
	switch name {
	case "referenceImages", "reference_images", "images", "imageUrls", "image_urls",
		"referenceVideos", "reference_videos", "videos", "videoUrls", "video_urls",
		"referenceAudios", "reference_audios", "audios", "audioUrls", "audio_urls":
		return true
	default:
		return false
	}
}

func validateMaiTokenMultipartUploads(submission *PPVideoSubmissionBody) error {
	if submission == nil || submission.Multipart == nil {
		return nil
	}
	var referenceFiles, firstFiles, lastFiles int
	for _, upload := range submission.Multipart.Uploads {
		switch upload.FieldName {
		case "referenceImages":
			referenceFiles++
		case "first_image":
			firstFiles++
		case "last_image":
			lastFiles++
		}
	}
	if firstFiles > 1 || firstFiles > 0 && gjson.GetBytes(submission.JSON, "first_image").Exists() {
		return fmt.Errorf("Mai Token Seedance accepts only one first frame")
	}
	if lastFiles > 1 || lastFiles > 0 && gjson.GetBytes(submission.JSON, "last_image").Exists() {
		return fmt.Errorf("Mai Token Seedance accepts only one last frame")
	}
	firstCount := firstFiles
	if gjson.GetBytes(submission.JSON, "first_image").Exists() {
		firstCount++
	}
	lastCount := lastFiles
	if gjson.GetBytes(submission.JSON, "last_image").Exists() {
		lastCount++
	}
	if lastCount > 0 && firstCount == 0 {
		return fmt.Errorf("Mai Token Seedance last_image requires first_image")
	}
	imageURLs := len(gjson.GetBytes(submission.JSON, "referenceImages").Array())
	videoURLs := len(gjson.GetBytes(submission.JSON, "referenceVideos").Array())
	audioURLs := len(gjson.GetBytes(submission.JSON, "referenceAudios").Array())
	if firstCount > 0 && referenceFiles+imageURLs+videoURLs+audioURLs > 0 {
		return fmt.Errorf("Mai Token Seedance cannot combine first/last frames with reference media")
	}
	if referenceFiles+imageURLs > maiTokenMaxReferenceImages {
		return fmt.Errorf("Mai Token Seedance accepts at most %d reference images", maiTokenMaxReferenceImages)
	}
	if referenceFiles+imageURLs+videoURLs+audioURLs > maiTokenMaxReferences {
		return fmt.Errorf("Mai Token Seedance accepts at most %d reference items", maiTokenMaxReferences)
	}
	return nil
}

func (b *PPVideoSubmissionBody) PayloadHash() string {
	if b == nil {
		return ""
	}
	if b.Multipart == nil {
		return HashUsageRequestPayload(b.JSON)
	}
	digest := sha256.New()
	maiTokenHashValue(digest, b.JSON)
	for index, upload := range b.Multipart.Uploads {
		maiTokenHashValue(digest, []byte(upload.FieldName))
		var ordinal [8]byte
		binary.BigEndian.PutUint64(ordinal[:], uint64(index))
		_, _ = digest.Write(ordinal[:])
		binary.BigEndian.PutUint64(ordinal[:], uint64(upload.Size))
		_, _ = digest.Write(ordinal[:])
		maiTokenHashValue(digest, []byte(upload.SHA256))
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func maiTokenHashValue(digest hash.Hash, value []byte) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = digest.Write(size[:])
	_, _ = digest.Write(value)
}

func (b *PPVideoSubmissionBody) Open() (io.ReadCloser, string, error) {
	if b == nil || len(b.JSON) == 0 {
		return nil, "", fmt.Errorf("PP video request body is empty")
	}
	if b.Multipart == nil {
		return io.NopCloser(bytes.NewReader(b.JSON)), "application/json", nil
	}
	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	go func() {
		writeErr := writeMaiTokenMultipartBody(writer, b.JSON, b.Multipart.Uploads)
		if closeErr := writer.Close(); writeErr == nil {
			writeErr = closeErr
		}
		_ = pipeWriter.CloseWithError(writeErr)
	}()
	return pipeReader, writer.FormDataContentType(), nil
}

func writeMaiTokenMultipartBody(writer *multipart.Writer, canonical []byte, uploads []MaiTokenUpload) error {
	var fields map[string]any
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.UseNumber()
	if err := decoder.Decode(&fields); err != nil {
		return fmt.Errorf("decode Mai Token Seedance canonical request: %w", err)
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		values, err := maiTokenMultipartOutputValues(fields[key])
		if err != nil {
			return fmt.Errorf("encode Mai Token Seedance field %s: %w", key, err)
		}
		for _, value := range values {
			if err := writer.WriteField(key, value); err != nil {
				return err
			}
		}
	}
	for _, upload := range uploads {
		file, err := os.Open(upload.TempPath)
		if err != nil {
			return err
		}
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{
			"name": upload.FieldName, "filename": upload.FileName,
		}))
		header.Set("Content-Type", upload.ContentType)
		part, createErr := writer.CreatePart(header)
		if createErr == nil {
			_, createErr = io.Copy(part, file)
		}
		closeErr := file.Close()
		if createErr != nil {
			return createErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func maiTokenMultipartOutputValues(value any) ([]string, error) {
	switch typed := value.(type) {
	case string:
		return []string{typed}, nil
	case bool:
		return []string{strconv.FormatBool(typed)}, nil
	case json.Number:
		return []string{typed.String()}, nil
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("array items must be strings")
			}
			values = append(values, text)
		}
		return values, nil
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		return []string{string(encoded)}, nil
	}
}

func (b *PPVideoSubmissionBody) Cleanup() error {
	if b == nil || b.Multipart == nil {
		return nil
	}
	b.Multipart.cleanupOnce.Do(func() {
		var cleanupErrors []error
		for _, upload := range b.Multipart.Uploads {
			if upload.TempPath == "" {
				continue
			}
			if err := os.Remove(upload.TempPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				cleanupErrors = append(cleanupErrors, err)
			}
		}
		b.Multipart.cleanupErr = errors.Join(cleanupErrors...)
	})
	return b.Multipart.cleanupErr
}
