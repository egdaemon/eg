// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        v6.31.1
// source: eg.interp.exec.proto

package execproxy

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type ExecRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Dir           string                 `protobuf:"bytes,1,opt,name=dir,proto3" json:"dir,omitempty"`
	Cmd           string                 `protobuf:"bytes,2,opt,name=cmd,proto3" json:"cmd,omitempty"`
	Arguments     []string               `protobuf:"bytes,3,rep,name=arguments,proto3" json:"arguments,omitempty"`
	Environment   []string               `protobuf:"bytes,4,rep,name=environment,proto3" json:"environment,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ExecRequest) Reset() {
	*x = ExecRequest{}
	mi := &file_eg_interp_exec_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ExecRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ExecRequest) ProtoMessage() {}

func (x *ExecRequest) ProtoReflect() protoreflect.Message {
	mi := &file_eg_interp_exec_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ExecRequest.ProtoReflect.Descriptor instead.
func (*ExecRequest) Descriptor() ([]byte, []int) {
	return file_eg_interp_exec_proto_rawDescGZIP(), []int{0}
}

func (x *ExecRequest) GetDir() string {
	if x != nil {
		return x.Dir
	}
	return ""
}

func (x *ExecRequest) GetCmd() string {
	if x != nil {
		return x.Cmd
	}
	return ""
}

func (x *ExecRequest) GetArguments() []string {
	if x != nil {
		return x.Arguments
	}
	return nil
}

func (x *ExecRequest) GetEnvironment() []string {
	if x != nil {
		return x.Environment
	}
	return nil
}

type ExecResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ExecResponse) Reset() {
	*x = ExecResponse{}
	mi := &file_eg_interp_exec_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ExecResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ExecResponse) ProtoMessage() {}

func (x *ExecResponse) ProtoReflect() protoreflect.Message {
	mi := &file_eg_interp_exec_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ExecResponse.ProtoReflect.Descriptor instead.
func (*ExecResponse) Descriptor() ([]byte, []int) {
	return file_eg_interp_exec_proto_rawDescGZIP(), []int{1}
}

var File_eg_interp_exec_proto protoreflect.FileDescriptor

const file_eg_interp_exec_proto_rawDesc = "" +
	"\n" +
	"\x14eg.interp.exec.proto\x12\x0eeg.interp.exec\"q\n" +
	"\vExecRequest\x12\x10\n" +
	"\x03dir\x18\x01 \x01(\tR\x03dir\x12\x10\n" +
	"\x03cmd\x18\x02 \x01(\tR\x03cmd\x12\x1c\n" +
	"\targuments\x18\x03 \x03(\tR\targuments\x12 \n" +
	"\venvironment\x18\x04 \x03(\tR\venvironment\"\x0e\n" +
	"\fExecResponse2L\n" +
	"\x05Proxy\x12C\n" +
	"\x04Exec\x12\x1b.eg.interp.exec.ExecRequest\x1a\x1c.eg.interp.exec.ExecResponse\"\x00b\x06proto3"

var (
	file_eg_interp_exec_proto_rawDescOnce sync.Once
	file_eg_interp_exec_proto_rawDescData []byte
)

func file_eg_interp_exec_proto_rawDescGZIP() []byte {
	file_eg_interp_exec_proto_rawDescOnce.Do(func() {
		file_eg_interp_exec_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_eg_interp_exec_proto_rawDesc), len(file_eg_interp_exec_proto_rawDesc)))
	})
	return file_eg_interp_exec_proto_rawDescData
}

var file_eg_interp_exec_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_eg_interp_exec_proto_goTypes = []any{
	(*ExecRequest)(nil),  // 0: eg.interp.exec.ExecRequest
	(*ExecResponse)(nil), // 1: eg.interp.exec.ExecResponse
}
var file_eg_interp_exec_proto_depIdxs = []int32{
	0, // 0: eg.interp.exec.Proxy.Exec:input_type -> eg.interp.exec.ExecRequest
	1, // 1: eg.interp.exec.Proxy.Exec:output_type -> eg.interp.exec.ExecResponse
	1, // [1:2] is the sub-list for method output_type
	0, // [0:1] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_eg_interp_exec_proto_init() }
func file_eg_interp_exec_proto_init() {
	if File_eg_interp_exec_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_eg_interp_exec_proto_rawDesc), len(file_eg_interp_exec_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_eg_interp_exec_proto_goTypes,
		DependencyIndexes: file_eg_interp_exec_proto_depIdxs,
		MessageInfos:      file_eg_interp_exec_proto_msgTypes,
	}.Build()
	File_eg_interp_exec_proto = out.File
	file_eg_interp_exec_proto_goTypes = nil
	file_eg_interp_exec_proto_depIdxs = nil
}
