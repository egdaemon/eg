package registration

//go:generate gomodifytags -file eg.actl.registration.pb.go -struct Registration -field Publickey -add-tags json:pkey -add-options json=omitempty -override -w -quiet
//go:generate gomodifytags -file eg.actl.registration.pb.go -struct Registration -field Cores -add-tags json:cores -add-options json=omitempty,json=string -override -w -quiet
//go:generate gomodifytags -file eg.actl.registration.pb.go -struct Registration -field Memory -add-tags json:memory -add-options json=omitempty,json=string -override -w -quiet
//go:generate gomodifytags -file eg.actl.registration.pb.go -struct Registration -field Vram -add-tags json:vram -add-options json=omitempty,json=string -override -w -quiet
//go:generate gomodifytags -file eg.actl.registration.pb.go -struct RegistrationGrantRequest -field Expiration -add-tags json:expiration -add-options json=omitempty,json=string -override -w -quiet
