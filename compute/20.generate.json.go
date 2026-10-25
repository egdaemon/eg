package compute

//go:generate gomodifytags -file eg.compute.history.pb.go -struct History -field Duration -add-tags json:duration -add-options json=omitempty,json=string -override -w -quiet
//go:generate gomodifytags -file eg.ci.authz.pb.go -struct Token -field Id -add-tags json:jti -add-options json=omitempty -override -w -quiet
//go:generate gomodifytags -file eg.ci.authz.pb.go -struct Token -field AccountId -add-tags json:iss -add-options json=omitempty -override -w -quiet
//go:generate gomodifytags -file eg.ci.authz.pb.go -struct Token -field ProfileId -add-tags json:sub -add-options json=omitempty -override -w -quiet
//go:generate gomodifytags -file eg.ci.authz.pb.go -struct Token -field SessionId -add-tags json:sid -add-options json=omitempty -override -w -quiet
//go:generate gomodifytags -file eg.ci.authz.pb.go -struct Token -field Issued -add-tags json:iat -add-options json=omitempty,json=string -override -w -quiet
//go:generate gomodifytags -file eg.ci.authz.pb.go -struct Token -field Expires -add-tags json:exp -add-options json=omitempty,json=string -override -w -quiet
//go:generate gomodifytags -file eg.ci.authz.pb.go -struct Token -field NotBefore -add-tags json:nbf -add-options json=omitempty,json=string -override -w -quiet
//go:generate gomodifytags -file eg.ci.authz.pb.go -struct Token -field ComputeRemaining -add-tags json:compute_remaining -add-options json=omitempty,json=string -override -w -quiet
