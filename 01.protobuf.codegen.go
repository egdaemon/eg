package eg

// go:generate protoc --proto_path=.proto --go_opt=Mauthn.proto=github.com/eg/authn --go_opt=paths=source_relative --go_out=authn authn.proto

//go:generate protoc --proto_path=.proto --go_opt=Meg.actl.registration.proto=github.com/eg/runners/registration --go_opt=paths=source_relative --go_out=runners/registration eg.actl.registration.proto
//go:generate protoc --proto_path=.proto --go_opt=Meg.actl.enqueued.proto=github.com/eg/runners --go_opt=paths=source_relative --go_out=runners eg.actl.enqueued.proto
//go:generate protoc --proto_path=.proto --go_opt=Meg.compute.proto=github.com/eg/compute --go_opt=paths=source_relative --go_out=compute eg.compute.proto
//go:generate protoc --proto_path=.proto --go_opt=Meg.compute.vcs.proto=github.com/eg/compute --go_opt=paths=source_relative --go_out=compute eg.compute.vcs.proto

//go:generate protoc --proto_path=.proto --go_opt=Meg.interp.events.proto=github.com/eg/interp/events --go_opt=paths=source_relative --go_out=interp/events eg.interp.events.proto
//go:generate protoc --proto_path=.proto --go-grpc_opt=Meg.interp.events.proto=github.com/eg/interp/events --go-grpc_opt=paths=source_relative --go-grpc_out=interp/events eg.interp.events.proto

//go:generate protoc --proto_path=.proto --go_opt=Meg.interp.containers.proto=github.com/eg/interp/c8s --go_opt=paths=source_relative --go_out=interp/c8s eg.interp.containers.proto
//go:generate protoc --proto_path=.proto --go-grpc_opt=Meg.interp.containers.proto=github.com/eg/interp/c8s --go-grpc_opt=paths=source_relative --go-grpc_out=interp/c8s eg.interp.containers.proto

//go:generate protoc --proto_path=.proto --go_opt=Meg.interp.exec.proto=github.com/eg/interp/exec --go_opt=paths=source_relative --go_out=interp/exec eg.interp.exec.proto
//go:generate protoc --proto_path=.proto --go-grpc_opt=Meg.interp.exec.proto=github.com/eg/interp/exec --go-grpc_opt=paths=source_relative --go-grpc_out=interp/exec eg.interp.exec.proto

//go:generate protoc --proto_path=.proto --go_opt=Meg.interp.fsnotify.proto=github.com/eg/interp/fsnotify --go_opt=paths=source_relative --go_out=interp/fsnotify eg.interp.fsnotify.proto
//go:generate protoc --proto_path=.proto --go-grpc_opt=Meg.interp.fsnotify.proto=github.com/eg/interp/fsnotify --go-grpc_opt=paths=source_relative --go-grpc_out=interp/fsnotify eg.interp.fsnotify.proto

//go:generate protoc --proto_path=.proto --go_opt=Mci.authz.proto=github.com/egciorg/eg/compute --go_opt=paths=source_relative --go_out=compute ci.authz.proto
