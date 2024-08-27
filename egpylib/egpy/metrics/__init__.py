from typing import Any
import logging
import json
import egpy.eg as eg
from .eg_interp_events_pb2_grpc import EventsStub

def record(name: str, m: Any):
  '''
  Records a metric payload with the given name. the name
  field is an opaque string identifying the metric.
  the payload must be consistent for its mapping of field names to types.
  the prefix 'eg.' is reserved for system use.
  i.e.) ✓ record("example.metric.1",
  i.e.) x record("eg.example.metric.1",
  metrics will be encoded as follows:
    - every event will be given a uuid v7 id.
    - every event will be given a timestamp.
    - name will be recorded as its literal value and as a hashed uuid.
    - timestamps must be encoded as ISO8601.
  '''

  with eg.autoclient() as c:
    try:
      logging.info(f"recording metric {name} initiated")
      EventsStub(c).Dispatch(
        eg.dispatch(eg.metric(name, json.dumps(m).encode('utf-8'))),
      )
    finally:
      logging.info(f"recording metric {name} completed")
