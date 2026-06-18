package events

import (
	"context"
	"database/sql"

	"github.com/egdaemon/eg/internal/errorsx"
	"google.golang.org/grpc"
)

func NewServiceDispatch(db *sql.DB) *EventsService {
	return &EventsService{
		db: db,
	}
}

type EventsService struct {
	UnimplementedEventsServer
	db *sql.DB
}

func (t *EventsService) Bind(host grpc.ServiceRegistrar) {
	RegisterEventsServer(host, t)
}

func (t *EventsService) Dispatch(ctx context.Context, dr *DispatchRequest) (_ *DispatchResponse, err error) {
	if err = RecordMetric(ctx, t.db, dr.Messages...); err != nil {
		return nil, errorsx.WithStack(err)
	}

	return &DispatchResponse{}, nil
}
