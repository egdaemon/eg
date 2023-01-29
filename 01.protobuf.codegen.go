package eg

//go:generate protoc --proto_path=.proto --go_opt=Meg.registration.proto=github.com/eg/registration --go_opt=paths=source_relative --go_out=registration eg.registration.proto
