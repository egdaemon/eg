package registration

import _ "github.com/mailru/easyjson"

//go:generate easyjson -snake_case -all -output_filename=eg.registration.pb.easyjson.gen.go eg.registration.pb.go