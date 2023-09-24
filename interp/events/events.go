package events

import (
	"context"
	"time"

	"github.com/gofrs/uuid"
)

func NewMessage(evt isMessage_Event) *Message {
	return &Message{
		Id:    uuid.Must(uuid.NewV7()).String(),
		Ts:    time.Now().Unix(),
		Event: evt,
	}
}

func NewPreambleV0(start time.Time, end time.Time) *Message {
	return NewMessage(&Message_Preamble{
		Preamble: &LogHeader{
			Major: 0,
			Minor: 0,
			Patch: 0,
			Sts:   start.Unix(),
			Ets:   end.Unix(),
		},
	})
}

func NewHeartbeat() *Message {
	return NewMessage(&Message_Heartbeat{
		Heartbeat: &Heartbeat{},
	})
}

func NewTask(t *Task) *Message {
	return NewMessage(&Message_Task{
		Task: t,
	})
}

func NewTaskPending(id, desc string) *Message {
	return NewTask(&Task{
		Id:          id,
		Description: desc,
		State:       Task_Pending,
	})
}

func NewTaskInitiated(id, desc string) *Message {
	return NewTask(&Task{
		Id:          id,
		Description: desc,
		State:       Task_Initiated,
	})
}

func NewTaskCompleted(id, desc string) *Message {
	return NewTask(&Task{
		Id:          id,
		Description: desc,
		State:       Task_Completed,
	})
}

func NewTaskErrored(id, desc string) *Message {
	return NewTask(&Task{
		Id:          id,
		Description: desc,
		State:       Task_Error,
	})
}

func NewLog(dir string) *Log {
	return &Log{
		dir:      dir,
		duration: 30 * time.Second,
	}
}

type Log struct {
	dir      string
	duration time.Duration
}

func (t *Log) Write(ctx context.Context, events ...*Message) error {
	return nil
}
