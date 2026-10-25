package computeapi

//go:generate gomodifytags -file eg.actl.enqueued.pb.go -struct Enqueued -field Cores -add-tags json:cores -add-options json=omitempty,json=string -override -w -quiet
//go:generate gomodifytags -file eg.actl.enqueued.pb.go -struct Enqueued -field Memory -add-tags json:memory -add-options json=omitempty,json=string -override -w -quiet
//go:generate gomodifytags -file eg.actl.enqueued.pb.go -struct Enqueued -field Vram -add-tags json:vram -add-options json=omitempty,json=string -override -w -quiet
//go:generate gomodifytags -file eg.actl.enqueued.pb.go -struct Enqueued -field Ttl -add-tags json:ttl -add-options json=omitempty,json=string -override -w -quiet
//go:generate gomodifytags -file eg.actl.enqueued.pb.go -struct Enqueued -field ClusterId -add-tags json:cid -add-options json=omitempty -override -w -quiet
//go:generate gomodifytags -file eg.actl.enqueued.pb.go -struct Enqueued -field Entry -add-tags json:entrypoint -add-options json=omitempty -override -w -quiet
//go:generate gomodifytags -file eg.actl.enqueued.pb.go -struct EnqueuedSearchRequest -field Offset -add-tags json:offset -add-options json=omitempty,json=string -override -w -quiet
//go:generate gomodifytags -file eg.actl.enqueued.pb.go -struct EnqueuedSearchRequest -field Limit -add-tags json:limit -add-options json=omitempty,json=string -override -w -quiet
//go:generate gomodifytags -file eg.actl.enqueued.pb.go -struct EnqueuedSearchRequest -field Cores -add-tags json:cores -add-options json=omitempty,json=string -override -w -quiet
//go:generate gomodifytags -file eg.actl.enqueued.pb.go -struct EnqueuedSearchRequest -field Memory -add-tags json:memory -add-options json=omitempty,json=string -override -w -quiet
//go:generate gomodifytags -file eg.actl.enqueued.pb.go -struct EnqueuedSearchRequest -field Vram -add-tags json:vram -add-options json=omitempty,json=string -override -w -quiet
//go:generate gomodifytags -file eg.actl.enqueued.pb.go -struct EnqueuedSearchRequest -field ClusterId -add-tags json:cid -add-options json=omitempty -override -w -quiet
//go:generate gomodifytags -file eg.actl.enqueued.pb.go -struct EnqueuedCreateRequest -field Enqueued -add-tags json:enqueue -add-options json=omitempty -override -w -quiet
//go:generate gomodifytags -file eg.actl.enqueued.pb.go -struct EnqueuedCreateResponse -field Enqueued -add-tags json:enqueue -add-options json=omitempty -override -w -quiet
//go:generate gomodifytags -file eg.actl.enqueued.pb.go -struct EnqueuedUpdateRequest -field Enqueued -add-tags json:enqueue -add-options json=omitempty -override -w -quiet
//go:generate gomodifytags -file eg.actl.enqueued.pb.go -struct EnqueuedUpdateResponse -field Enqueued -add-tags json:enqueue -add-options json=omitempty -override -w -quiet
//go:generate gomodifytags -file eg.actl.enqueued.pb.go -struct EnqueuedCompletedResponse -field Enqueued -add-tags json:enqueue -add-options json=omitempty -override -w -quiet
//go:generate gomodifytags -file eg.actl.enqueued.pb.go -struct EnqueuedFindRequest -field Enqueued -add-tags json:enqueue -add-options json=omitempty -override -w -quiet
//go:generate gomodifytags -file eg.actl.enqueued.pb.go -struct EnqueuedFindResponse -field Enqueued -add-tags json:enqueue -add-options json=omitempty -override -w -quiet
