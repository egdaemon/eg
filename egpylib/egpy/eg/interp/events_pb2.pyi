from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class RunMetadata(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: bytes
    def __init__(self, id: _Optional[bytes] = ...) -> None: ...

class LogHeader(_message.Message):
    __slots__ = ("Major", "Minor", "Patch", "sts", "ets")
    MAJOR_FIELD_NUMBER: _ClassVar[int]
    MINOR_FIELD_NUMBER: _ClassVar[int]
    PATCH_FIELD_NUMBER: _ClassVar[int]
    STS_FIELD_NUMBER: _ClassVar[int]
    ETS_FIELD_NUMBER: _ClassVar[int]
    Major: int
    Minor: int
    Patch: int
    sts: int
    ets: int
    def __init__(self, Major: _Optional[int] = ..., Minor: _Optional[int] = ..., Patch: _Optional[int] = ..., sts: _Optional[int] = ..., ets: _Optional[int] = ...) -> None: ...

class Heartbeat(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class Task(_message.Message):
    __slots__ = ("id", "pid", "description", "state", "deadline")
    class State(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        Pending: _ClassVar[Task.State]
        Initiated: _ClassVar[Task.State]
        Completed: _ClassVar[Task.State]
        Error: _ClassVar[Task.State]
    Pending: Task.State
    Initiated: Task.State
    Completed: Task.State
    Error: Task.State
    ID_FIELD_NUMBER: _ClassVar[int]
    PID_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    DEADLINE_FIELD_NUMBER: _ClassVar[int]
    id: str
    pid: str
    description: str
    state: Task.State
    deadline: int
    def __init__(self, id: _Optional[str] = ..., pid: _Optional[str] = ..., description: _Optional[str] = ..., state: _Optional[_Union[Task.State, str]] = ..., deadline: _Optional[int] = ...) -> None: ...

class Metric(_message.Message):
    __slots__ = ("name", "fieldsJSON")
    NAME_FIELD_NUMBER: _ClassVar[int]
    FIELDSJSON_FIELD_NUMBER: _ClassVar[int]
    name: str
    fieldsJSON: bytes
    def __init__(self, name: _Optional[str] = ..., fieldsJSON: _Optional[bytes] = ...) -> None: ...

class Message(_message.Message):
    __slots__ = ("id", "ts", "preamble", "heartbeat", "task", "metric")
    ID_FIELD_NUMBER: _ClassVar[int]
    TS_FIELD_NUMBER: _ClassVar[int]
    PREAMBLE_FIELD_NUMBER: _ClassVar[int]
    HEARTBEAT_FIELD_NUMBER: _ClassVar[int]
    TASK_FIELD_NUMBER: _ClassVar[int]
    METRIC_FIELD_NUMBER: _ClassVar[int]
    id: str
    ts: int
    preamble: LogHeader
    heartbeat: Heartbeat
    task: Task
    metric: Metric
    def __init__(self, id: _Optional[str] = ..., ts: _Optional[int] = ..., preamble: _Optional[_Union[LogHeader, _Mapping]] = ..., heartbeat: _Optional[_Union[Heartbeat, _Mapping]] = ..., task: _Optional[_Union[Task, _Mapping]] = ..., metric: _Optional[_Union[Metric, _Mapping]] = ...) -> None: ...

class RunUploadChunk(_message.Message):
    __slots__ = ("data", "checksum", "none", "metadata")
    class Metadata(_message.Message):
        __slots__ = ("bytes", "checksum")
        BYTES_FIELD_NUMBER: _ClassVar[int]
        CHECKSUM_FIELD_NUMBER: _ClassVar[int]
        bytes: int
        checksum: bytes
        def __init__(self, bytes: _Optional[int] = ..., checksum: _Optional[bytes] = ...) -> None: ...
    DATA_FIELD_NUMBER: _ClassVar[int]
    CHECKSUM_FIELD_NUMBER: _ClassVar[int]
    NONE_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    data: bytes
    checksum: bytes
    none: bool
    metadata: RunUploadChunk.Metadata
    def __init__(self, data: _Optional[bytes] = ..., checksum: _Optional[bytes] = ..., none: bool = ..., metadata: _Optional[_Union[RunUploadChunk.Metadata, _Mapping]] = ...) -> None: ...

class RunUploadResponse(_message.Message):
    __slots__ = ("run",)
    RUN_FIELD_NUMBER: _ClassVar[int]
    run: RunMetadata
    def __init__(self, run: _Optional[_Union[RunMetadata, _Mapping]] = ...) -> None: ...

class RunLogRequest(_message.Message):
    __slots__ = ("run",)
    RUN_FIELD_NUMBER: _ClassVar[int]
    run: RunMetadata
    def __init__(self, run: _Optional[_Union[RunMetadata, _Mapping]] = ...) -> None: ...

class RunLogResponse(_message.Message):
    __slots__ = ("content",)
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    content: bytes
    def __init__(self, content: _Optional[bytes] = ...) -> None: ...

class RunInitiateRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class RunInitiateResult(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class RunCancelRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class RunCancelResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class RunWatchRequest(_message.Message):
    __slots__ = ("run",)
    RUN_FIELD_NUMBER: _ClassVar[int]
    run: RunMetadata
    def __init__(self, run: _Optional[_Union[RunMetadata, _Mapping]] = ...) -> None: ...

class DispatchRequest(_message.Message):
    __slots__ = ("messages",)
    MESSAGES_FIELD_NUMBER: _ClassVar[int]
    messages: _containers.RepeatedCompositeFieldContainer[Message]
    def __init__(self, messages: _Optional[_Iterable[_Union[Message, _Mapping]]] = ...) -> None: ...

class DispatchResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...
