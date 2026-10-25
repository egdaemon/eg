package computeapi

import (
	"encoding/json"
	"mime"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewDirectUploadDescription(t *testing.T) {
	mimetype, body, err := NewDirectUpload(
		&EnqueuedDequeueResponse{Enqueued: &Enqueued{Id: "pushed-id", Description: "my workload"}},
		"candidate-token",
		strings.NewReader("kernel contents"),
		strings.NewReader("environ contents"),
	)
	require.NoError(t, err)
	defer body.Close()

	_, params, err := mime.ParseMediaType(mimetype)
	require.NoError(t, err)

	form, err := multipart.NewReader(body, params["boundary"]).ReadForm(1 << 20)
	require.NoError(t, err)
	defer form.RemoveAll()

	require.Len(t, form.Value["enqueued"], 1)

	var decoded EnqueuedDequeueResponse
	require.NoError(t, json.Unmarshal([]byte(form.Value["enqueued"][0]), &decoded))
	require.Equal(t, "my workload", decoded.Enqueued.Description)
}
