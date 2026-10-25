package schedules

//go:generate gomodifytags -file eg.repository.schedule.pb.go -struct Schedule -field Entry -add-tags json:entrypoint -add-options json=omitempty -override -w -quiet
//go:generate gomodifytags -file eg.repository.schedule.pb.go -struct Schedule -field Cores -add-tags json:cores -add-options json=omitempty,json=string -override -w -quiet
//go:generate gomodifytags -file eg.repository.schedule.pb.go -struct Schedule -field Memory -add-tags json:memory -add-options json=omitempty,json=string -override -w -quiet
//go:generate gomodifytags -file eg.repository.schedule.pb.go -struct Schedule -field Vram -add-tags json:vram -add-options json=omitempty,json=string -override -w -quiet
//go:generate gomodifytags -file eg.repository.schedule.pb.go -struct Schedule -field Ttl -add-tags json:ttl -add-options json=omitempty,json=string -override -w -quiet
//go:generate gomodifytags -file eg.repository.schedule.pb.go -struct ScheduleSearchRequest -field Offset -add-tags json:offset -add-options json=omitempty,json=string -override -w -quiet
//go:generate gomodifytags -file eg.repository.schedule.pb.go -struct ScheduleSearchRequest -field Limit -add-tags json:limit -add-options json=omitempty,json=string -override -w -quiet
