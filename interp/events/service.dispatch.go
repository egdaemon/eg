package events

import (
	"context"

	"github.com/pkg/errors"
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
		return nil, errors.WithStack(err)
	}

	return &DispatchResponse{}, nil
}
