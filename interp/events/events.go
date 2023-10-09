package events

import (
	"context"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	sync "sync"
	"time"

	"github.com/gofrs/uuid"
	"github.com/james-lawrence/eg/internal/errorsx"
	"github.com/james-lawrence/eg/internal/iox"
	"github.com/james-lawrence/eg/internal/protobuflog"
)

const (
	format = "2006.01.02.15.04.05.log"
)

func NewMessage(evt isMessage_Event) *Message {
	return &Message{
		Id:    uuid.Must(uuid.NewV7()).String(),
		Ts:    time.Now().UnixMicro(),
		Event: evt,
	}
}

func NewPreambleV0(start time.Time, end time.Time) *Message {
	return NewMessage(&Message_Preamble{
		Preamble: &LogHeader{
			Major: 0,
			Minor: 0,
			Patch: 0,
			Sts:   start.UnixMicro(),
			Ets:   end.UnixMicro(),
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

func NewTaskInitiated(pid, id, desc string) *Message {
	return NewTask(&Task{
		Id:          id,
		Pid:         pid,
		Description: desc,
		State:       Task_Initiated,
	})
}

func NewTaskCompleted(pid, id, desc string) *Message {
	return NewTask(&Task{
		Id:          id,
		Pid:         pid,
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

func NewDispatch(m ...*Message) *DispatchRequest {
	return &DispatchRequest{Messages: m}
}

func NewLog(dir string) *Log {
	return &Log{
		dir:      dir,
		duration: 30 * time.Second,
		l:        &sync.Mutex{},
	}
}

func NewLogEnsureDir(dir string) (_ *Log, err error) {
	if err = os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	return NewLog(dir), nil
}

func NewLogDirFromRun(root string, md *RunMetadata) string {
	return NewLogDirFromRunID(root, uuid.FromBytesOrNil(md.Id).String())
}

func NewLogDirFromRunID(root string, id string) string {
	return filepath.Join(root, id, "events")
}

type Log struct {
	dir      string
	duration time.Duration
	l        *sync.Mutex
}

func (t *Log) Write(ctx context.Context, events ...*Message) error {
	var (
		fh      *os.File
		current string
		encoder *protobuflog.Encoder[*Message]
	)
	t.l.Lock()
	defer t.l.Unlock()

	replacefh := func(old io.Closer, path string) (_ *os.File, err error) {
		errorsx.MaybeLog(errorsx.Wrap(iox.MaybeClose(old), "unable to close log file"))
		return os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	}

	for _, e := range events {
		var (
			err error
		)

		logname := filepath.Join(t.dir, time.UnixMicro(e.Ts).Truncate(t.duration).Format(format))
		if logname != current {
			if fh, err = replacefh(fh, logname); err != nil {
				return err
			}
			current = logname
			encoder = protobuflog.NewEncoder[*Message](fh)
		}

		if err = encoder.Encode(e); err != nil {
			return err
		}
	}

	return iox.MaybeClose(fh)
}

func detectFirstLogTimestamp(dir string) time.Time {
	var (
		ts = time.Now()
	)
	err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if path == dir {
			return nil
		}

		if info.IsDir() {
			return filepath.SkipDir
		}

		if ts, err = time.Parse(format, info.Name()); err != nil {
			return err
		}

		return filepath.SkipAll
	})

	if err != nil {
		log.Println("unable to detect first log", err)
		return time.Now()
	}

	return ts
}

func NewReader(dir string) *Reader {
	return &Reader{
		dir:       dir,
		duration:  30 * time.Second,
		current:   detectFirstLogTimestamp(dir),
		batchSize: 100,
	}
}

type Reader struct {
	dir       string
	duration  time.Duration
	current   time.Time
	batchSize int
}

func (t *Reader) Read(ctx context.Context, dst *[]*Message) (err error) {
	var (
		fh      *os.File
		decoder *protobuflog.Decoder[*Message]
		buf     = make([]*Message, 0, t.batchSize)
	)

	replacefh := func(old *os.File, path string) (_ *os.File, err error) {
		errorsx.MaybeLog(errorsx.Wrap(iox.MaybeClose(old), "unable to close log file"))
		return os.Open(path)
	}

	logname := filepath.Join(t.dir, t.current.Truncate(t.duration).Format(format))

	if fh, err = replacefh(fh, logname); os.IsNotExist(err) {
		return io.EOF
	} else if err != nil {
		return err
	}
	decoder = protobuflog.NewDecoder[*Message](fh)

	if err = decoder.Decode(&buf); err != nil {
		return err
	}

	*dst = append(*dst, buf...)

	if len(buf) < cap(buf) && time.Since(t.current) > t.duration {
		t.current = t.current.Add(t.duration)
	}

	return nil
}
