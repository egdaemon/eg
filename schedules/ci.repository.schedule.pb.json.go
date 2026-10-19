package schedules

import "google.golang.org/protobuf/encoding/protojson"

// unknown fields are discarded so fields added to the control plane's messages do not break released providers.
var decoder = protojson.UnmarshalOptions{DiscardUnknown: true}

func (t *Schedule) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *Schedule) UnmarshalJSON(b []byte) error { return decoder.Unmarshal(b, t) }

func (t *ScheduleSearchRequest) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *ScheduleSearchRequest) UnmarshalJSON(b []byte) error { return decoder.Unmarshal(b, t) }

func (t *ScheduleSearchResponse) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *ScheduleSearchResponse) UnmarshalJSON(b []byte) error { return decoder.Unmarshal(b, t) }

func (t *ScheduleCreateRequest) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *ScheduleCreateRequest) UnmarshalJSON(b []byte) error { return decoder.Unmarshal(b, t) }

func (t *ScheduleCreateResponse) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *ScheduleCreateResponse) UnmarshalJSON(b []byte) error { return decoder.Unmarshal(b, t) }

func (t *ScheduleUpdateRequest) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *ScheduleUpdateRequest) UnmarshalJSON(b []byte) error { return decoder.Unmarshal(b, t) }

func (t *ScheduleUpdateResponse) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *ScheduleUpdateResponse) UnmarshalJSON(b []byte) error { return decoder.Unmarshal(b, t) }

func (t *ScheduleFindResponse) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *ScheduleFindResponse) UnmarshalJSON(b []byte) error { return decoder.Unmarshal(b, t) }

func (t *ScheduleDestroyRequest) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *ScheduleDestroyRequest) UnmarshalJSON(b []byte) error { return decoder.Unmarshal(b, t) }

func (t *ScheduleDestroyResponse) MarshalJSON() ([]byte, error) { return protojson.Marshal(t) }
func (t *ScheduleDestroyResponse) UnmarshalJSON(b []byte) error { return decoder.Unmarshal(b, t) }
