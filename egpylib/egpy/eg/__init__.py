import os
import time
import grpc
from . import interp
from uuid_extensions import uuid7str

def defaultRunnerRuntimeDir() -> str:
	return os.environ.get("EG_RUNTIME_DIRECTORY", os.path.join("/", "opt", "egruntime"))

def defaultRunnerSocketPath() -> str:
	return os.path.join(defaultRunnerRuntimeDir(), "control.socket")

def autoclient():
	return grpc.insecure_channel(f"unix://{defaultRunnerSocketPath()}")

def metric(name: str, fields: bytes):
		return interp.events_pb2.Message(
			id=uuid7str(),
			ts=time.time_ns() // 1000000,
			metric=interp.events_pb2.Metric(name=name, fieldsJSON=fields)
		)

def dispatch(*arg: interp.events_pb2.Message) -> interp.events_pb2.DispatchRequest:
	return interp.events_pb2.DispatchRequest(messages=arg)
