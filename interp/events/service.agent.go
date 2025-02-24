package events

import (
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/gofrs/uuid"
	"google.golang.org/grpc"
)

func NewServiceAgent(root string) *AgentService {
	return &AgentService{
		dir: root,
	}
}

type AgentService struct {
	UnimplementedAgentServer
	dir string
}

func (t *AgentService) Bind(host grpc.ServiceRegistrar) {
	RegisterAgentServer(host, t)
}

// Upload implements RunServer.
func (t *AgentService) Upload(s Agent_UploadServer) (err error) {
	var (
		uid   uuid.UUID
		chunk *RunUploadChunk
	)

	if chunk, err = s.Recv(); err != nil {
		return errorsx.WithStack(err)
	}

	if uid, err = uuid.NewV7(); err != nil {
		return errorsx.WithStack(err)
	}

	metadata := chunk.GetMetadata()
	dst, err := os.Create(filepath.Join(t.dir, md5x.FormatHex(md5x.Digest(metadata.Checksum))))
	if err != nil {
		return errorsx.WithStack(err)
	}

	for {
		chunk, err := s.Recv()

		if err == io.EOF {
			return s.SendAndClose(&RunUploadResponse{
				Run: &RunMetadata{
					Id: uid.Bytes(),
				},
			})
		}

		if err != nil {
			log.Println("error receiving chunk", err)
			return err
		}

		if _, err = io.Copy(dst, bytes.NewBuffer(chunk.Data)); err != nil {
			log.Println("error uploading chunk", err)
			return err
		}
	}
}

func (*AgentService) Initiate(ctx context.Context, evt *RunInitiateRequest) (*RunInitiateResult, error) {
	panic("unimplemented")
}

func (*AgentService) Cancel(ctx context.Context, evt *RunCancelRequest) (*RunCancelResponse, error) {
	panic("unimplemented")
}

func (t *AgentService) Logs(l *RunLogRequest, s Agent_LogsServer) (err error) {
	r := NewReader(
		NewLogDirFromRun(t.dir, l.Run),
	)

	for err == nil {
		var (
			buf = make([]*Message, 0, 5)
		)

		if err = r.Read(s.Context(), &buf); err != nil {
			continue
		}

		if err = s.Send(&RunLogResponse{}); err != nil {
			continue
		}
	}

	return err
}

func (t *AgentService) Watch(rw *RunWatchRequest, s Agent_WatchServer) (err error) {
	r := NewReader(
		NewLogDirFromRun(t.dir, rw.Run),
	)

	for err == nil {
		var (
			buf = make([]*Message, 0, 5)
		)

		select {
		case <-s.Context().Done():
			return s.Context().Err()
		default:
		}

		if cause := r.Read(s.Context(), &buf); cause == io.EOF {
			time.Sleep(time.Second)
			continue
		} else if cause != nil {
			err = cause
			continue
		}

		for _, m := range buf {
			if err = errorsx.WithStack(s.Send(m)); err != nil {
				continue
			}
		}
	}

	return err
}
