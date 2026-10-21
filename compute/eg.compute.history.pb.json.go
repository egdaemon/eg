package compute

import "google.golang.org/protobuf/encoding/protojson"

func (t *History) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *History) UnmarshalJSON(b []byte) error { return protojson.Unmarshal(b, t) }

func (t *HistoryFindResponse) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *HistoryFindResponse) UnmarshalJSON(b []byte) error { return protojson.Unmarshal(b, t) }
