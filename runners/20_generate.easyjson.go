package runners

import _ "github.com/mailru/easyjson"

//go:generate easyjson -snake_case -all -omit_empty -output_filename=eg.actl.enqueued.pb.easyjson.gen.go eg.actl.enqueued.pb.go
