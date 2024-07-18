package events

import (
	"context"
	"database/sql"

	"github.com/egdaemon/eg/internal/errorsx"
	"google.golang.org/grpc"
)

func NewServiceDispatch(logger *Log, db *sql.DB) *EventsService {
	return &EventsService{
		logger: logger,
		db:     db,
	}
}

type EventsService struct {
	UnimplementedEventsServer
	logger *Log
	db     *sql.DB
}

func (t *EventsService) Bind(host grpc.ServiceRegistrar) {
	RegisterEventsServer(host, t)
}

func (t *EventsService) Dispatch(ctx context.Context, dr *DispatchRequest) (_ *DispatchResponse, err error) {
	// if err = t.logger.Write(ctx, dr.Messages...); err != nil {
	// 	return nil, errorsx.WithStack(err)
	// }

	if err = RecordMetric(ctx, t.db, dr.Messages...); err != nil {
		return nil, errorsx.WithStack(err)
	}

	return &DispatchResponse{}, nil
}
