package runners

import "google.golang.org/protobuf/encoding/protojson"

func (t *Enqueued) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *Enqueued) UnmarshalJSON(b []byte) error { return protojson.Unmarshal(b, t) }

func (t *EnqueuedCompletedRequest) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *EnqueuedCompletedRequest) UnmarshalJSON(b []byte) error { return protojson.Unmarshal(b, t) }

func (t *EnqueuedCompletedResponse) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *EnqueuedCompletedResponse) UnmarshalJSON(b []byte) error { return protojson.Unmarshal(b, t) }

func (t *EnqueuedCreateRequest) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *EnqueuedCreateRequest) UnmarshalJSON(b []byte) error { return protojson.Unmarshal(b, t) }

func (t *EnqueuedCreateResponse) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *EnqueuedCreateResponse) UnmarshalJSON(b []byte) error { return protojson.Unmarshal(b, t) }

func (t *EnqueuedDequeueResponse) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *EnqueuedDequeueResponse) UnmarshalJSON(b []byte) error { return protojson.Unmarshal(b, t) }

func (t *EnqueuedDownloadRequest) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *EnqueuedDownloadRequest) UnmarshalJSON(b []byte) error { return protojson.Unmarshal(b, t) }

func (t *EnqueuedFindRequest) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *EnqueuedFindRequest) UnmarshalJSON(b []byte) error { return protojson.Unmarshal(b, t) }

func (t *EnqueuedFindResponse) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *EnqueuedFindResponse) UnmarshalJSON(b []byte) error { return protojson.Unmarshal(b, t) }

func (t *EnqueuedSearchRequest) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *EnqueuedSearchRequest) UnmarshalJSON(b []byte) error { return protojson.Unmarshal(b, t) }

func (t *EnqueuedSearchResponse) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *EnqueuedSearchResponse) UnmarshalJSON(b []byte) error { return protojson.Unmarshal(b, t) }

func (t *EnqueuedUpdateRequest) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *EnqueuedUpdateRequest) UnmarshalJSON(b []byte) error { return protojson.Unmarshal(b, t) }

func (t *EnqueuedUpdateResponse) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *EnqueuedUpdateResponse) UnmarshalJSON(b []byte) error { return protojson.Unmarshal(b, t) }
