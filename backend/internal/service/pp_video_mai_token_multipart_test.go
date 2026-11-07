package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type maiTokenMultipartTestUpload struct {
	Field       string
	Name        string
	ContentType string
	Body        string
}

func newMaiTokenMultipartTestRequest(
	t *testing.T,
	boundary string,
	fields map[string][]string,
	uploads []maiTokenMultipartTestUpload,
) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.SetBoundary(boundary))
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		for _, value := range fields[key] {
			require.NoError(t, writer.WriteField(key, value))
		}
	}
	for _, upload := range uploads {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{
			"name":     upload.Field,
			"filename": upload.Name,
		}))
		header.Set("Content-Type", upload.ContentType)
		part, err := writer.CreatePart(header)
		require.NoError(t, err)
		_, err = io.WriteString(part, upload.Body)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	req, err := http.NewRequest(http.MethodPost, "/v1/videos/generations", bytes.NewReader(body.Bytes()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestPrepareMaiTokenMultipartSubmissionSpoolsAndHashesFiles(t *testing.T) {
	fields := map[string][]string{
		"model":          {MaiTokenSeedanceModel},
		"prompt":         {"lake"},
		"duration":       {"4"},
		"generate_audio": {"false"},
	}
	uploads := []maiTokenMultipartTestUpload{{
		Field: "first_image", Name: "first.jpg", ContentType: "image/jpeg", Body: "jpeg-bytes",
	}}
	reqA := newMaiTokenMultipartTestRequest(t, "boundary-a", fields, uploads)
	reqB := newMaiTokenMultipartTestRequest(t, "boundary-b", fields, uploads)

	a, err := PrepareMaiTokenMultipartSubmission(reqA)
	require.NoError(t, err)
	t.Cleanup(func() { _ = a.Cleanup() })
	b, err := PrepareMaiTokenMultipartSubmission(reqB)
	require.NoError(t, err)
	t.Cleanup(func() { _ = b.Cleanup() })

	require.Equal(t, a.PayloadHash(), b.PayloadHash())
	require.JSONEq(t, `{
		"model":"sdtj-2.5-480p",
		"prompt":"lake",
		"duration":4,
		"resolution":"480p",
		"generate_audio":false
	}`, string(a.JSON))
	require.Len(t, a.Multipart.Uploads, 1)
	upload := a.Multipart.Uploads[0]
	require.Equal(t, "first_image", upload.FieldName)
	require.Equal(t, "first.jpg", upload.FileName)
	require.Equal(t, "image/jpeg", upload.ContentType)
	require.Equal(t, int64(len("jpeg-bytes")), upload.Size)
	digest := sha256.Sum256([]byte("jpeg-bytes"))
	require.Equal(t, hex.EncodeToString(digest[:]), upload.SHA256)
	_, err = os.Stat(upload.TempPath)
	require.NoError(t, err)

	opened, contentType, err := a.Open()
	require.NoError(t, err)
	t.Cleanup(func() { _ = opened.Close() })
	mediaType, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType)
	reader := multipart.NewReader(opened, params["boundary"])
	seenFields := map[string][]string{}
	var seenUpload maiTokenMultipartTestUpload
	for {
		part, partErr := reader.NextPart()
		if partErr == io.EOF {
			break
		}
		require.NoError(t, partErr)
		data, readErr := io.ReadAll(part)
		require.NoError(t, readErr)
		if part.FileName() == "" {
			seenFields[part.FormName()] = append(seenFields[part.FormName()], string(data))
			continue
		}
		seenUpload = maiTokenMultipartTestUpload{
			Field: part.FormName(), Name: part.FileName(), ContentType: part.Header.Get("Content-Type"), Body: string(data),
		}
	}
	require.Equal(t, []string{"sdtj-2.5-480p"}, seenFields["model"])
	require.Equal(t, []string{"lake"}, seenFields["prompt"])
	require.Equal(t, []string{"4"}, seenFields["duration"])
	require.Equal(t, []string{"480p"}, seenFields["resolution"])
	require.Equal(t, []string{"false"}, seenFields["generate_audio"])
	require.Equal(t, uploads[0], seenUpload)

	require.NoError(t, a.Cleanup())
	_, err = os.Stat(upload.TempPath)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestPrepareMaiTokenMultipartSubmissionPreservesRepeatedReferenceImages(t *testing.T) {
	req := newMaiTokenMultipartTestRequest(t, "reference-boundary", map[string][]string{
		"model":           {MaiTokenSeedanceModel},
		"prompt":          {"lake"},
		"referenceImages": {"https://example.com/url.jpg"},
	}, []maiTokenMultipartTestUpload{
		{Field: "referenceImages", Name: "one.jpg", ContentType: "image/jpeg", Body: "one"},
		{Field: "referenceImages", Name: "two.png", ContentType: "image/png", Body: "two"},
	})

	submission, err := PrepareMaiTokenMultipartSubmission(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = submission.Cleanup() })
	require.Len(t, submission.Multipart.Uploads, 2)
	require.Equal(t, "one.jpg", submission.Multipart.Uploads[0].FileName)
	require.Equal(t, "two.png", submission.Multipart.Uploads[1].FileName)
	require.JSONEq(t, `{
		"model":"sdtj-2.5-480p",
		"prompt":"lake",
		"duration":4,
		"resolution":"480p",
		"referenceImages":["https://example.com/url.jpg"]
	}`, string(submission.JSON))
}

func TestPrepareMaiTokenMultipartSubmissionRejectsDisallowedFiles(t *testing.T) {
	tests := []string{"referenceVideos", "referenceAudios", "input_reference", "unknown"}
	for _, field := range tests {
		field := field
		t.Run(field, func(t *testing.T) {
			tempDir := t.TempDir()
			t.Setenv("TMPDIR", tempDir)
			req := newMaiTokenMultipartTestRequest(t, "rejected-boundary", map[string][]string{
				"model": {MaiTokenSeedanceModel}, "prompt": {"lake"},
			}, []maiTokenMultipartTestUpload{{Field: field, Name: "media.bin", ContentType: "application/octet-stream", Body: "bytes"}})

			_, err := PrepareMaiTokenMultipartSubmission(req)
			require.ErrorContains(t, err, field)
			matches, globErr := filepath.Glob(filepath.Join(tempDir, "sub2api-mai-token-*"))
			require.NoError(t, globErr)
			require.Empty(t, matches)
		})
	}
}

func TestPrepareMaiTokenMultipartSubmissionCleansSpoolOnValidationFailure(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TMPDIR", tempDir)
	req := newMaiTokenMultipartTestRequest(t, "cleanup-boundary", map[string][]string{
		"model": {MaiTokenSeedanceModel}, "prompt": {"lake"}, "duration": {"3"},
	}, []maiTokenMultipartTestUpload{{Field: "first_image", Name: "first.jpg", ContentType: "image/jpeg", Body: "bytes"}})

	_, err := PrepareMaiTokenMultipartSubmission(req)
	require.ErrorContains(t, err, "duration")
	entries, readErr := os.ReadDir(tempDir)
	require.NoError(t, readErr)
	for _, entry := range entries {
		require.False(t, strings.HasPrefix(entry.Name(), "sub2api-mai-token-"), entry.Name())
	}
}

func TestPrepareMaiTokenMultipartSubmissionRejectsFrameReferenceMixing(t *testing.T) {
	req := newMaiTokenMultipartTestRequest(t, "mixed-boundary", map[string][]string{
		"model": {MaiTokenSeedanceModel}, "prompt": {"lake"},
	}, []maiTokenMultipartTestUpload{
		{Field: "first_image", Name: "first.jpg", ContentType: "image/jpeg", Body: "first"},
		{Field: "referenceImages", Name: "reference.jpg", ContentType: "image/jpeg", Body: "reference"},
	})

	_, err := PrepareMaiTokenMultipartSubmission(req)
	require.ErrorContains(t, err, "cannot combine")
}
