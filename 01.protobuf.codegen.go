package eg

//go:generate protoc --proto_path=.proto --go_opt=Meg.actl.registration.proto=github.com/eg/registration --go_opt=paths=source_relative --go_out=registration eg.actl.registration.proto
//go:generate protoc --proto_path=.proto --go_opt=Mauthn.proto=github.com/eg/authn --go_opt=paths=source_relative --go_out=authn authn.proto
//go:generate protoc --proto_path=.proto --go_opt=Meg.interp.events.proto=github.com/eg/interp/events --go_opt=paths=source_relative --go_out=interp/events eg.interp.events.proto
