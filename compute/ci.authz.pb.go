// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        v6.31.1
// source: ci.authz.proto

package compute

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

type Authorization struct {
	state            protoimpl.MessageState `protogen:"open.v1"`
	ComputeRead      bool                   `protobuf:"varint,1,opt,name=compute_read,proto3" json:"compute_read,omitempty"`
	ComputeModify    bool                   `protobuf:"varint,2,opt,name=compute_modify,proto3" json:"compute_modify,omitempty"`
	QuotaRead        bool                   `protobuf:"varint,3,opt,name=quota_read,proto3" json:"quota_read,omitempty"`
	QuotaModify      bool                   `protobuf:"varint,4,opt,name=quota_modify,proto3" json:"quota_modify,omitempty"`
	RepositoryRead   bool                   `protobuf:"varint,5,opt,name=repository_read,proto3" json:"repository_read,omitempty"`
	RepositoryModify bool                   `protobuf:"varint,6,opt,name=repository_modify,proto3" json:"repository_modify,omitempty"`
	ComputeShared    bool                   `protobuf:"varint,7,opt,name=compute_shared,proto3" json:"compute_shared,omitempty"`
	ComputeRemaining uint64                 `protobuf:"varint,8,opt,name=compute_remaining,proto3" json:"compute_remaining,omitempty"`
	unknownFields    protoimpl.UnknownFields
	sizeCache        protoimpl.SizeCache
}

func (x *Authorization) Reset() {
	*x = Authorization{}
	mi := &file_ci_authz_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Authorization) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Authorization) ProtoMessage() {}

func (x *Authorization) ProtoReflect() protoreflect.Message {
	mi := &file_ci_authz_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Authorization.ProtoReflect.Descriptor instead.
func (*Authorization) Descriptor() ([]byte, []int) {
	return file_ci_authz_proto_rawDescGZIP(), []int{0}
}

func (x *Authorization) GetComputeRead() bool {
	if x != nil {
		return x.ComputeRead
	}
	return false
}

func (x *Authorization) GetComputeModify() bool {
	if x != nil {
		return x.ComputeModify
	}
	return false
}

func (x *Authorization) GetQuotaRead() bool {
	if x != nil {
		return x.QuotaRead
	}
	return false
}

func (x *Authorization) GetQuotaModify() bool {
	if x != nil {
		return x.QuotaModify
	}
	return false
}

func (x *Authorization) GetRepositoryRead() bool {
	if x != nil {
		return x.RepositoryRead
	}
	return false
}

func (x *Authorization) GetRepositoryModify() bool {
	if x != nil {
		return x.RepositoryModify
	}
	return false
}

func (x *Authorization) GetComputeShared() bool {
	if x != nil {
		return x.ComputeShared
	}
	return false
}

func (x *Authorization) GetComputeRemaining() uint64 {
	if x != nil {
		return x.ComputeRemaining
	}
	return 0
}

type Token struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// START OF STANDARD FIELDS
	Id               string `protobuf:"bytes,1,opt,name=id,json=jti,proto3" json:"id,omitempty"`
	AccountId        string `protobuf:"bytes,2,opt,name=account_id,json=iss,proto3" json:"account_id,omitempty"`
	ProfileId        string `protobuf:"bytes,3,opt,name=profile_id,json=sub,proto3" json:"profile_id,omitempty"`
	SessionId        string `protobuf:"bytes,4,opt,name=session_id,json=sid,proto3" json:"session_id,omitempty"`
	Issued           int64  `protobuf:"varint,5,opt,name=issued,json=iat,proto3" json:"issued,omitempty"`
	Expires          int64  `protobuf:"varint,6,opt,name=expires,json=exp,proto3" json:"expires,omitempty"`
	NotBefore        int64  `protobuf:"varint,7,opt,name=not_before,json=nbf,proto3" json:"not_before,omitempty"`
	Bearer           string `protobuf:"bytes,8,opt,name=bearer,proto3" json:"bearer,omitempty"`
	ComputeRead      bool   `protobuf:"varint,1000,opt,name=compute_read,proto3" json:"compute_read,omitempty"`
	ComputeModify    bool   `protobuf:"varint,1002,opt,name=compute_modify,proto3" json:"compute_modify,omitempty"`
	QuotaRead        bool   `protobuf:"varint,1003,opt,name=quota_read,proto3" json:"quota_read,omitempty"`
	QuotaModify      bool   `protobuf:"varint,1004,opt,name=quota_modify,proto3" json:"quota_modify,omitempty"`
	RepositoryRead   bool   `protobuf:"varint,1005,opt,name=repository_read,proto3" json:"repository_read,omitempty"`
	RepositoryModify bool   `protobuf:"varint,1006,opt,name=repository_modify,proto3" json:"repository_modify,omitempty"`
	// compute_shared is a highly sensitive field used to authorize access to any
	// workload task. should never be *written* to in the code base, only manually
	// changed in the DB.
	ComputeShared    bool   `protobuf:"varint,1007,opt,name=compute_shared,proto3" json:"compute_shared,omitempty"`
	ComputeRemaining uint64 `protobuf:"varint,1008,opt,name=compute_remaining,proto3" json:"compute_remaining,omitempty"`
	unknownFields    protoimpl.UnknownFields
	sizeCache        protoimpl.SizeCache
}

func (x *Token) Reset() {
	*x = Token{}
	mi := &file_ci_authz_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Token) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Token) ProtoMessage() {}

func (x *Token) ProtoReflect() protoreflect.Message {
	mi := &file_ci_authz_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Token.ProtoReflect.Descriptor instead.
func (*Token) Descriptor() ([]byte, []int) {
	return file_ci_authz_proto_rawDescGZIP(), []int{1}
}

func (x *Token) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Token) GetAccountId() string {
	if x != nil {
		return x.AccountId
	}
	return ""
}

func (x *Token) GetProfileId() string {
	if x != nil {
		return x.ProfileId
	}
	return ""
}

func (x *Token) GetSessionId() string {
	if x != nil {
		return x.SessionId
	}
	return ""
}

func (x *Token) GetIssued() int64 {
	if x != nil {
		return x.Issued
	}
	return 0
}

func (x *Token) GetExpires() int64 {
	if x != nil {
		return x.Expires
	}
	return 0
}

func (x *Token) GetNotBefore() int64 {
	if x != nil {
		return x.NotBefore
	}
	return 0
}

func (x *Token) GetBearer() string {
	if x != nil {
		return x.Bearer
	}
	return ""
}

func (x *Token) GetComputeRead() bool {
	if x != nil {
		return x.ComputeRead
	}
	return false
}

func (x *Token) GetComputeModify() bool {
	if x != nil {
		return x.ComputeModify
	}
	return false
}

func (x *Token) GetQuotaRead() bool {
	if x != nil {
		return x.QuotaRead
	}
	return false
}

func (x *Token) GetQuotaModify() bool {
	if x != nil {
		return x.QuotaModify
	}
	return false
}

func (x *Token) GetRepositoryRead() bool {
	if x != nil {
		return x.RepositoryRead
	}
	return false
}

func (x *Token) GetRepositoryModify() bool {
	if x != nil {
		return x.RepositoryModify
	}
	return false
}

func (x *Token) GetComputeShared() bool {
	if x != nil {
		return x.ComputeShared
	}
	return false
}

func (x *Token) GetComputeRemaining() uint64 {
	if x != nil {
		return x.ComputeRemaining
	}
	return 0
}

type AuthzRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *AuthzRequest) Reset() {
	*x = AuthzRequest{}
	mi := &file_ci_authz_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *AuthzRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AuthzRequest) ProtoMessage() {}

func (x *AuthzRequest) ProtoReflect() protoreflect.Message {
	mi := &file_ci_authz_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AuthzRequest.ProtoReflect.Descriptor instead.
func (*AuthzRequest) Descriptor() ([]byte, []int) {
	return file_ci_authz_proto_rawDescGZIP(), []int{2}
}

type AuthzResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Token         *Token                 `protobuf:"bytes,1,opt,name=token,proto3" json:"token,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *AuthzResponse) Reset() {
	*x = AuthzResponse{}
	mi := &file_ci_authz_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *AuthzResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AuthzResponse) ProtoMessage() {}

func (x *AuthzResponse) ProtoReflect() protoreflect.Message {
	mi := &file_ci_authz_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AuthzResponse.ProtoReflect.Descriptor instead.
func (*AuthzResponse) Descriptor() ([]byte, []int) {
	return file_ci_authz_proto_rawDescGZIP(), []int{3}
}

func (x *AuthzResponse) GetToken() *Token {
	if x != nil {
		return x.Token
	}
	return nil
}

type GrantRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ProfileId     string                 `protobuf:"bytes,1,opt,name=profile_id,proto3" json:"profile_id,omitempty"`
	Authorization *Authorization         `protobuf:"bytes,2,opt,name=authorization,proto3" json:"authorization,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GrantRequest) Reset() {
	*x = GrantRequest{}
	mi := &file_ci_authz_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GrantRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GrantRequest) ProtoMessage() {}

func (x *GrantRequest) ProtoReflect() protoreflect.Message {
	mi := &file_ci_authz_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GrantRequest.ProtoReflect.Descriptor instead.
func (*GrantRequest) Descriptor() ([]byte, []int) {
	return file_ci_authz_proto_rawDescGZIP(), []int{4}
}

func (x *GrantRequest) GetProfileId() string {
	if x != nil {
		return x.ProfileId
	}
	return ""
}

func (x *GrantRequest) GetAuthorization() *Authorization {
	if x != nil {
		return x.Authorization
	}
	return nil
}

type GrantResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ProfileId     string                 `protobuf:"bytes,1,opt,name=profile_id,proto3" json:"profile_id,omitempty"`
	Authorization *Authorization         `protobuf:"bytes,2,opt,name=authorization,proto3" json:"authorization,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GrantResponse) Reset() {
	*x = GrantResponse{}
	mi := &file_ci_authz_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GrantResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GrantResponse) ProtoMessage() {}

func (x *GrantResponse) ProtoReflect() protoreflect.Message {
	mi := &file_ci_authz_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GrantResponse.ProtoReflect.Descriptor instead.
func (*GrantResponse) Descriptor() ([]byte, []int) {
	return file_ci_authz_proto_rawDescGZIP(), []int{5}
}

func (x *GrantResponse) GetProfileId() string {
	if x != nil {
		return x.ProfileId
	}
	return ""
}

func (x *GrantResponse) GetAuthorization() *Authorization {
	if x != nil {
		return x.Authorization
	}
	return nil
}

type RevokeRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ProfileId     string                 `protobuf:"bytes,1,opt,name=profile_id,proto3" json:"profile_id,omitempty"`
	Authorization *Authorization         `protobuf:"bytes,2,opt,name=authorization,proto3" json:"authorization,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *RevokeRequest) Reset() {
	*x = RevokeRequest{}
	mi := &file_ci_authz_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RevokeRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RevokeRequest) ProtoMessage() {}

func (x *RevokeRequest) ProtoReflect() protoreflect.Message {
	mi := &file_ci_authz_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RevokeRequest.ProtoReflect.Descriptor instead.
func (*RevokeRequest) Descriptor() ([]byte, []int) {
	return file_ci_authz_proto_rawDescGZIP(), []int{6}
}

func (x *RevokeRequest) GetProfileId() string {
	if x != nil {
		return x.ProfileId
	}
	return ""
}

func (x *RevokeRequest) GetAuthorization() *Authorization {
	if x != nil {
		return x.Authorization
	}
	return nil
}

type RevokeResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ProfileId     string                 `protobuf:"bytes,1,opt,name=profile_id,proto3" json:"profile_id,omitempty"`
	Authorization *Authorization         `protobuf:"bytes,2,opt,name=authorization,proto3" json:"authorization,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *RevokeResponse) Reset() {
	*x = RevokeResponse{}
	mi := &file_ci_authz_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RevokeResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RevokeResponse) ProtoMessage() {}

func (x *RevokeResponse) ProtoReflect() protoreflect.Message {
	mi := &file_ci_authz_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RevokeResponse.ProtoReflect.Descriptor instead.
func (*RevokeResponse) Descriptor() ([]byte, []int) {
	return file_ci_authz_proto_rawDescGZIP(), []int{7}
}

func (x *RevokeResponse) GetProfileId() string {
	if x != nil {
		return x.ProfileId
	}
	return ""
}

func (x *RevokeResponse) GetAuthorization() *Authorization {
	if x != nil {
		return x.Authorization
	}
	return nil
}

type ProfileRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	ProfileId     string                 `protobuf:"bytes,1,opt,name=profile_id,proto3" json:"profile_id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ProfileRequest) Reset() {
	*x = ProfileRequest{}
	mi := &file_ci_authz_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ProfileRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ProfileRequest) ProtoMessage() {}

func (x *ProfileRequest) ProtoReflect() protoreflect.Message {
	mi := &file_ci_authz_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ProfileRequest.ProtoReflect.Descriptor instead.
func (*ProfileRequest) Descriptor() ([]byte, []int) {
	return file_ci_authz_proto_rawDescGZIP(), []int{8}
}

func (x *ProfileRequest) GetProfileId() string {
	if x != nil {
		return x.ProfileId
	}
	return ""
}

type ProfileResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Authorization *Authorization         `protobuf:"bytes,1,opt,name=authorization,proto3" json:"authorization,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ProfileResponse) Reset() {
	*x = ProfileResponse{}
	mi := &file_ci_authz_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ProfileResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ProfileResponse) ProtoMessage() {}

func (x *ProfileResponse) ProtoReflect() protoreflect.Message {
	mi := &file_ci_authz_proto_msgTypes[9]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ProfileResponse.ProtoReflect.Descriptor instead.
func (*ProfileResponse) Descriptor() ([]byte, []int) {
	return file_ci_authz_proto_rawDescGZIP(), []int{9}
}

func (x *ProfileResponse) GetAuthorization() *Authorization {
	if x != nil {
		return x.Authorization
	}
	return nil
}

var File_ci_authz_proto protoreflect.FileDescriptor

const file_ci_authz_proto_rawDesc = "" +
	"\n" +
	"\x0eci.authz.proto\x12\bci.authz\"\xcd\x02\n" +
	"\rAuthorization\x12\"\n" +
	"\fcompute_read\x18\x01 \x01(\bR\fcompute_read\x12&\n" +
	"\x0ecompute_modify\x18\x02 \x01(\bR\x0ecompute_modify\x12\x1e\n" +
	"\n" +
	"quota_read\x18\x03 \x01(\bR\n" +
	"quota_read\x12\"\n" +
	"\fquota_modify\x18\x04 \x01(\bR\fquota_modify\x12(\n" +
	"\x0frepository_read\x18\x05 \x01(\bR\x0frepository_read\x12,\n" +
	"\x11repository_modify\x18\x06 \x01(\bR\x11repository_modify\x12&\n" +
	"\x0ecompute_shared\x18\a \x01(\bR\x0ecompute_shared\x12,\n" +
	"\x11compute_remaining\x18\b \x01(\x04R\x11compute_remaining\"\x8c\x04\n" +
	"\x05Token\x12\x0f\n" +
	"\x02id\x18\x01 \x01(\tR\x03jti\x12\x17\n" +
	"\n" +
	"account_id\x18\x02 \x01(\tR\x03iss\x12\x17\n" +
	"\n" +
	"profile_id\x18\x03 \x01(\tR\x03sub\x12\x17\n" +
	"\n" +
	"session_id\x18\x04 \x01(\tR\x03sid\x12\x13\n" +
	"\x06issued\x18\x05 \x01(\x03R\x03iat\x12\x14\n" +
	"\aexpires\x18\x06 \x01(\x03R\x03exp\x12\x17\n" +
	"\n" +
	"not_before\x18\a \x01(\x03R\x03nbf\x12\x16\n" +
	"\x06bearer\x18\b \x01(\tR\x06bearer\x12#\n" +
	"\fcompute_read\x18\xe8\a \x01(\bR\fcompute_read\x12'\n" +
	"\x0ecompute_modify\x18\xea\a \x01(\bR\x0ecompute_modify\x12\x1f\n" +
	"\n" +
	"quota_read\x18\xeb\a \x01(\bR\n" +
	"quota_read\x12#\n" +
	"\fquota_modify\x18\xec\a \x01(\bR\fquota_modify\x12)\n" +
	"\x0frepository_read\x18\xed\a \x01(\bR\x0frepository_read\x12-\n" +
	"\x11repository_modify\x18\xee\a \x01(\bR\x11repository_modify\x12'\n" +
	"\x0ecompute_shared\x18\xef\a \x01(\bR\x0ecompute_shared\x12-\n" +
	"\x11compute_remaining\x18\xf0\a \x01(\x04R\x11compute_remainingJ\x05\b\t\x10\xe8\a\"\x0e\n" +
	"\fAuthzRequest\"6\n" +
	"\rAuthzResponse\x12%\n" +
	"\x05token\x18\x01 \x01(\v2\x0f.ci.authz.TokenR\x05token\"m\n" +
	"\fGrantRequest\x12\x1e\n" +
	"\n" +
	"profile_id\x18\x01 \x01(\tR\n" +
	"profile_id\x12=\n" +
	"\rauthorization\x18\x02 \x01(\v2\x17.ci.authz.AuthorizationR\rauthorization\"n\n" +
	"\rGrantResponse\x12\x1e\n" +
	"\n" +
	"profile_id\x18\x01 \x01(\tR\n" +
	"profile_id\x12=\n" +
	"\rauthorization\x18\x02 \x01(\v2\x17.ci.authz.AuthorizationR\rauthorization\"n\n" +
	"\rRevokeRequest\x12\x1e\n" +
	"\n" +
	"profile_id\x18\x01 \x01(\tR\n" +
	"profile_id\x12=\n" +
	"\rauthorization\x18\x02 \x01(\v2\x17.ci.authz.AuthorizationR\rauthorization\"o\n" +
	"\x0eRevokeResponse\x12\x1e\n" +
	"\n" +
	"profile_id\x18\x01 \x01(\tR\n" +
	"profile_id\x12=\n" +
	"\rauthorization\x18\x02 \x01(\v2\x17.ci.authz.AuthorizationR\rauthorization\"0\n" +
	"\x0eProfileRequest\x12\x1e\n" +
	"\n" +
	"profile_id\x18\x01 \x01(\tR\n" +
	"profile_id\"P\n" +
	"\x0fProfileResponse\x12=\n" +
	"\rauthorization\x18\x01 \x01(\v2\x17.ci.authz.AuthorizationR\rauthorizationb\x06proto3"

var (
	file_ci_authz_proto_rawDescOnce sync.Once
	file_ci_authz_proto_rawDescData []byte
)

func file_ci_authz_proto_rawDescGZIP() []byte {
	file_ci_authz_proto_rawDescOnce.Do(func() {
		file_ci_authz_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_ci_authz_proto_rawDesc), len(file_ci_authz_proto_rawDesc)))
	})
	return file_ci_authz_proto_rawDescData
}

var file_ci_authz_proto_msgTypes = make([]protoimpl.MessageInfo, 10)
var file_ci_authz_proto_goTypes = []any{
	(*Authorization)(nil),   // 0: ci.authz.Authorization
	(*Token)(nil),           // 1: ci.authz.Token
	(*AuthzRequest)(nil),    // 2: ci.authz.AuthzRequest
	(*AuthzResponse)(nil),   // 3: ci.authz.AuthzResponse
	(*GrantRequest)(nil),    // 4: ci.authz.GrantRequest
	(*GrantResponse)(nil),   // 5: ci.authz.GrantResponse
	(*RevokeRequest)(nil),   // 6: ci.authz.RevokeRequest
	(*RevokeResponse)(nil),  // 7: ci.authz.RevokeResponse
	(*ProfileRequest)(nil),  // 8: ci.authz.ProfileRequest
	(*ProfileResponse)(nil), // 9: ci.authz.ProfileResponse
}
var file_ci_authz_proto_depIdxs = []int32{
	1, // 0: ci.authz.AuthzResponse.token:type_name -> ci.authz.Token
	0, // 1: ci.authz.GrantRequest.authorization:type_name -> ci.authz.Authorization
	0, // 2: ci.authz.GrantResponse.authorization:type_name -> ci.authz.Authorization
	0, // 3: ci.authz.RevokeRequest.authorization:type_name -> ci.authz.Authorization
	0, // 4: ci.authz.RevokeResponse.authorization:type_name -> ci.authz.Authorization
	0, // 5: ci.authz.ProfileResponse.authorization:type_name -> ci.authz.Authorization
	6, // [6:6] is the sub-list for method output_type
	6, // [6:6] is the sub-list for method input_type
	6, // [6:6] is the sub-list for extension type_name
	6, // [6:6] is the sub-list for extension extendee
	0, // [0:6] is the sub-list for field type_name
}

func init() { file_ci_authz_proto_init() }
func file_ci_authz_proto_init() {
	if File_ci_authz_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_ci_authz_proto_rawDesc), len(file_ci_authz_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   10,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_ci_authz_proto_goTypes,
		DependencyIndexes: file_ci_authz_proto_depIdxs,
		MessageInfos:      file_ci_authz_proto_msgTypes,
	}.Build()
	File_ci_authz_proto = out.File
	file_ci_authz_proto_goTypes = nil
	file_ci_authz_proto_depIdxs = nil
}
