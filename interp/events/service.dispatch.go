package events

import (
	"context"

	"github.com/egdaemon/eg/internal/errorsx"
	grpc "google.golang.org/grpc"
)

func NewServiceDispatch(logger *Log) *EventsService {
	return &EventsService{
		logger: logger,
	}
}

type EventsService struct {
	UnimplementedEventsServer
	logger *Log
}

func (t *EventsService) Bind(host grpc.ServiceRegistrar) {
	RegisterEventsServer(host, t)
}

func (t *EventsService) Dispatch(ctx context.Context, dr *DispatchRequest) (_ *DispatchResponse, err error) {
	if err = t.logger.Write(ctx, dr.Messages...); err != nil {
		return nil, errorsx.WithStack(err)
	}

	return &DispatchResponse{}, nil
}
