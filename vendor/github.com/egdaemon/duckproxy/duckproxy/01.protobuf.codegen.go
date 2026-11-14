package duckproxy

//go:generate protoc --proto_path=../.proto --go_opt=Mduckproxy.proto=github.com/egdaemon/duckproxy/duckproxy --go_opt=paths=source_relative --go_out=. duckproxy.proto
