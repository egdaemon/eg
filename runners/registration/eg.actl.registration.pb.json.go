package registration

import (
	"google.golang.org/protobuf/encoding/protojson"
)

func (x *PingRequest) MarshalJSON() ([]byte, error) {
	return protojson.Marshal(x)
}

func (x *PingRequest) UnmarshalJSON(b []byte) error {
	return protojson.Unmarshal(b, x)
}

func (x *PingResponse) MarshalJSON() ([]byte, error) {
	return protojson.Marshal(x)
}

func (x *PingResponse) UnmarshalJSON(b []byte) error {
	return protojson.Unmarshal(b, x)
}

func (x *Registration) MarshalJSON() ([]byte, error) {
	return protojson.Marshal(x)
}

func (x *Registration) UnmarshalJSON(b []byte) error {
	return protojson.Unmarshal(b, x)
}

func (x *RegistrationGrantRequest) MarshalJSON() ([]byte, error) {
	return protojson.Marshal(x)
}

func (x *RegistrationGrantRequest) UnmarshalJSON(b []byte) error {
	return protojson.Unmarshal(b, x)
}

func (x *RegistrationGrantResponse) MarshalJSON() ([]byte, error) {
	return protojson.Marshal(x)
}

func (x *RegistrationGrantResponse) UnmarshalJSON(b []byte) error {
	return protojson.Unmarshal(b, x)
}

func (x *RegistrationRequest) MarshalJSON() ([]byte, error) {
	return protojson.Marshal(x)
}

func (x *RegistrationRequest) UnmarshalJSON(b []byte) error {
	return protojson.Unmarshal(b, x)
}

func (x *RegistrationResponse) MarshalJSON() ([]byte, error) {
	return protojson.Marshal(x)
}

func (x *RegistrationResponse) UnmarshalJSON(b []byte) error {
	return protojson.Unmarshal(b, x)
}

func (x *RegistrationSearchRequest) MarshalJSON() ([]byte, error) {
	return protojson.Marshal(x)
}

func (x *RegistrationSearchRequest) UnmarshalJSON(b []byte) error {
	return protojson.Unmarshal(b, x)
}

func (x *RegistrationSearchResponse) MarshalJSON() ([]byte, error) {
	return protojson.Marshal(x)
}

func (x *RegistrationSearchResponse) UnmarshalJSON(b []byte) error {
	return protojson.Unmarshal(b, x)
}
