// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        (unknown)
// source: rpc_airport.proto

package pb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type CreateAirportRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name        string  `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	IataCode    string  `protobuf:"bytes,2,opt,name=iata_code,json=iataCode,proto3" json:"iata_code,omitempty"`
	IcaoCode    string  `protobuf:"bytes,3,opt,name=icao_code,json=icaoCode,proto3" json:"icao_code,omitempty"`
	City        string  `protobuf:"bytes,4,opt,name=city,proto3" json:"city,omitempty"`
	CountryCode string  `protobuf:"bytes,5,opt,name=country_code,json=countryCode,proto3" json:"country_code,omitempty"`
	State       string  `protobuf:"bytes,6,opt,name=state,proto3" json:"state,omitempty"`
	Lat         float64 `protobuf:"fixed64,7,opt,name=lat,proto3" json:"lat,omitempty"`
	Long        float64 `protobuf:"fixed64,8,opt,name=long,proto3" json:"long,omitempty"`
	Elevation   *string `protobuf:"bytes,9,opt,name=elevation,proto3,oneof" json:"elevation,omitempty"`
	Timezone    string  `protobuf:"bytes,10,opt,name=timezone,proto3" json:"timezone,omitempty"`
}

func (x *CreateAirportRequest) Reset() {
	*x = CreateAirportRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_rpc_airport_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateAirportRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateAirportRequest) ProtoMessage() {}

func (x *CreateAirportRequest) ProtoReflect() protoreflect.Message {
	mi := &file_rpc_airport_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateAirportRequest.ProtoReflect.Descriptor instead.
func (*CreateAirportRequest) Descriptor() ([]byte, []int) {
	return file_rpc_airport_proto_rawDescGZIP(), []int{0}
}

func (x *CreateAirportRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *CreateAirportRequest) GetIataCode() string {
	if x != nil {
		return x.IataCode
	}
	return ""
}

func (x *CreateAirportRequest) GetIcaoCode() string {
	if x != nil {
		return x.IcaoCode
	}
	return ""
}

func (x *CreateAirportRequest) GetCity() string {
	if x != nil {
		return x.City
	}
	return ""
}

func (x *CreateAirportRequest) GetCountryCode() string {
	if x != nil {
		return x.CountryCode
	}
	return ""
}

func (x *CreateAirportRequest) GetState() string {
	if x != nil {
		return x.State
	}
	return ""
}

func (x *CreateAirportRequest) GetLat() float64 {
	if x != nil {
		return x.Lat
	}
	return 0
}

func (x *CreateAirportRequest) GetLong() float64 {
	if x != nil {
		return x.Long
	}
	return 0
}

func (x *CreateAirportRequest) GetElevation() string {
	if x != nil && x.Elevation != nil {
		return *x.Elevation
	}
	return ""
}

func (x *CreateAirportRequest) GetTimezone() string {
	if x != nil {
		return x.Timezone
	}
	return ""
}

type Airport struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name        string  `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	IataCode    string  `protobuf:"bytes,2,opt,name=iata_code,json=iataCode,proto3" json:"iata_code,omitempty"`
	IcaoCode    string  `protobuf:"bytes,3,opt,name=icao_code,json=icaoCode,proto3" json:"icao_code,omitempty"`
	City        string  `protobuf:"bytes,4,opt,name=city,proto3" json:"city,omitempty"`
	CountryCode string  `protobuf:"bytes,5,opt,name=country_code,json=countryCode,proto3" json:"country_code,omitempty"`
	State       string  `protobuf:"bytes,6,opt,name=state,proto3" json:"state,omitempty"`
	Lat         float64 `protobuf:"fixed64,7,opt,name=lat,proto3" json:"lat,omitempty"`
	Long        float64 `protobuf:"fixed64,8,opt,name=long,proto3" json:"long,omitempty"`
	Elevation   *string `protobuf:"bytes,9,opt,name=elevation,proto3,oneof" json:"elevation,omitempty"`
	Timezone    string  `protobuf:"bytes,10,opt,name=timezone,proto3" json:"timezone,omitempty"`
}

func (x *Airport) Reset() {
	*x = Airport{}
	if protoimpl.UnsafeEnabled {
		mi := &file_rpc_airport_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Airport) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Airport) ProtoMessage() {}

func (x *Airport) ProtoReflect() protoreflect.Message {
	mi := &file_rpc_airport_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Airport.ProtoReflect.Descriptor instead.
func (*Airport) Descriptor() ([]byte, []int) {
	return file_rpc_airport_proto_rawDescGZIP(), []int{1}
}

func (x *Airport) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Airport) GetIataCode() string {
	if x != nil {
		return x.IataCode
	}
	return ""
}

func (x *Airport) GetIcaoCode() string {
	if x != nil {
		return x.IcaoCode
	}
	return ""
}

func (x *Airport) GetCity() string {
	if x != nil {
		return x.City
	}
	return ""
}

func (x *Airport) GetCountryCode() string {
	if x != nil {
		return x.CountryCode
	}
	return ""
}

func (x *Airport) GetState() string {
	if x != nil {
		return x.State
	}
	return ""
}

func (x *Airport) GetLat() float64 {
	if x != nil {
		return x.Lat
	}
	return 0
}

func (x *Airport) GetLong() float64 {
	if x != nil {
		return x.Long
	}
	return 0
}

func (x *Airport) GetElevation() string {
	if x != nil && x.Elevation != nil {
		return *x.Elevation
	}
	return ""
}

func (x *Airport) GetTimezone() string {
	if x != nil {
		return x.Timezone
	}
	return ""
}

type CreateAirportResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Airport *Airport `protobuf:"bytes,1,opt,name=airport,proto3" json:"airport,omitempty"`
}

func (x *CreateAirportResponse) Reset() {
	*x = CreateAirportResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_rpc_airport_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateAirportResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateAirportResponse) ProtoMessage() {}

func (x *CreateAirportResponse) ProtoReflect() protoreflect.Message {
	mi := &file_rpc_airport_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateAirportResponse.ProtoReflect.Descriptor instead.
func (*CreateAirportResponse) Descriptor() ([]byte, []int) {
	return file_rpc_airport_proto_rawDescGZIP(), []int{2}
}

func (x *CreateAirportResponse) GetAirport() *Airport {
	if x != nil {
		return x.Airport
	}
	return nil
}

type ListAirportResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Airport []*Airport `protobuf:"bytes,1,rep,name=airport,proto3" json:"airport,omitempty"`
}

func (x *ListAirportResponse) Reset() {
	*x = ListAirportResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_rpc_airport_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListAirportResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListAirportResponse) ProtoMessage() {}

func (x *ListAirportResponse) ProtoReflect() protoreflect.Message {
	mi := &file_rpc_airport_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListAirportResponse.ProtoReflect.Descriptor instead.
func (*ListAirportResponse) Descriptor() ([]byte, []int) {
	return file_rpc_airport_proto_rawDescGZIP(), []int{3}
}

func (x *ListAirportResponse) GetAirport() []*Airport {
	if x != nil {
		return x.Airport
	}
	return nil
}

type ListAirportRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Limit  int64 `protobuf:"varint,1,opt,name=limit,proto3" json:"limit,omitempty"`
	Offset int64 `protobuf:"varint,2,opt,name=offset,proto3" json:"offset,omitempty"`
}

func (x *ListAirportRequest) Reset() {
	*x = ListAirportRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_rpc_airport_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListAirportRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListAirportRequest) ProtoMessage() {}

func (x *ListAirportRequest) ProtoReflect() protoreflect.Message {
	mi := &file_rpc_airport_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListAirportRequest.ProtoReflect.Descriptor instead.
func (*ListAirportRequest) Descriptor() ([]byte, []int) {
	return file_rpc_airport_proto_rawDescGZIP(), []int{4}
}

func (x *ListAirportRequest) GetLimit() int64 {
	if x != nil {
		return x.Limit
	}
	return 0
}

func (x *ListAirportRequest) GetOffset() int64 {
	if x != nil {
		return x.Offset
	}
	return 0
}

type GetAirportRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id       int64   `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	Name     *string `protobuf:"bytes,2,opt,name=name,proto3,oneof" json:"name,omitempty"`
	IataCode *string `protobuf:"bytes,3,opt,name=iata_code,json=iataCode,proto3,oneof" json:"iata_code,omitempty"`
	IcaoCode *string `protobuf:"bytes,4,opt,name=icao_code,json=icaoCode,proto3,oneof" json:"icao_code,omitempty"`
}

func (x *GetAirportRequest) Reset() {
	*x = GetAirportRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_rpc_airport_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetAirportRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetAirportRequest) ProtoMessage() {}

func (x *GetAirportRequest) ProtoReflect() protoreflect.Message {
	mi := &file_rpc_airport_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetAirportRequest.ProtoReflect.Descriptor instead.
func (*GetAirportRequest) Descriptor() ([]byte, []int) {
	return file_rpc_airport_proto_rawDescGZIP(), []int{5}
}

func (x *GetAirportRequest) GetId() int64 {
	if x != nil {
		return x.Id
	}
	return 0
}

func (x *GetAirportRequest) GetName() string {
	if x != nil && x.Name != nil {
		return *x.Name
	}
	return ""
}

func (x *GetAirportRequest) GetIataCode() string {
	if x != nil && x.IataCode != nil {
		return *x.IataCode
	}
	return ""
}

func (x *GetAirportRequest) GetIcaoCode() string {
	if x != nil && x.IcaoCode != nil {
		return *x.IcaoCode
	}
	return ""
}

type GetAirportResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Airport *Airport `protobuf:"bytes,1,opt,name=airport,proto3" json:"airport,omitempty"`
}

func (x *GetAirportResponse) Reset() {
	*x = GetAirportResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_rpc_airport_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetAirportResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetAirportResponse) ProtoMessage() {}

func (x *GetAirportResponse) ProtoReflect() protoreflect.Message {
	mi := &file_rpc_airport_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetAirportResponse.ProtoReflect.Descriptor instead.
func (*GetAirportResponse) Descriptor() ([]byte, []int) {
	return file_rpc_airport_proto_rawDescGZIP(), []int{6}
}

func (x *GetAirportResponse) GetAirport() *Airport {
	if x != nil {
		return x.Airport
	}
	return nil
}

type UpdateAirportRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id          int64    `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	Name        *string  `protobuf:"bytes,2,opt,name=name,proto3,oneof" json:"name,omitempty"`
	IataCode    *string  `protobuf:"bytes,3,opt,name=iata_code,json=iataCode,proto3,oneof" json:"iata_code,omitempty"`
	IcaoCode    *string  `protobuf:"bytes,4,opt,name=icao_code,json=icaoCode,proto3,oneof" json:"icao_code,omitempty"`
	City        *string  `protobuf:"bytes,5,opt,name=city,proto3,oneof" json:"city,omitempty"`
	CountryCode *string  `protobuf:"bytes,6,opt,name=country_code,json=countryCode,proto3,oneof" json:"country_code,omitempty"`
	State       *string  `protobuf:"bytes,7,opt,name=state,proto3,oneof" json:"state,omitempty"`
	Lat         *float64 `protobuf:"fixed64,8,opt,name=lat,proto3,oneof" json:"lat,omitempty"`
	Long        *float64 `protobuf:"fixed64,9,opt,name=long,proto3,oneof" json:"long,omitempty"`
	Elevation   *string  `protobuf:"bytes,10,opt,name=elevation,proto3,oneof" json:"elevation,omitempty"`
	Timezone    *string  `protobuf:"bytes,11,opt,name=timezone,proto3,oneof" json:"timezone,omitempty"`
}

func (x *UpdateAirportRequest) Reset() {
	*x = UpdateAirportRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_rpc_airport_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateAirportRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateAirportRequest) ProtoMessage() {}

func (x *UpdateAirportRequest) ProtoReflect() protoreflect.Message {
	mi := &file_rpc_airport_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateAirportRequest.ProtoReflect.Descriptor instead.
func (*UpdateAirportRequest) Descriptor() ([]byte, []int) {
	return file_rpc_airport_proto_rawDescGZIP(), []int{7}
}

func (x *UpdateAirportRequest) GetId() int64 {
	if x != nil {
		return x.Id
	}
	return 0
}

func (x *UpdateAirportRequest) GetName() string {
	if x != nil && x.Name != nil {
		return *x.Name
	}
	return ""
}

func (x *UpdateAirportRequest) GetIataCode() string {
	if x != nil && x.IataCode != nil {
		return *x.IataCode
	}
	return ""
}

func (x *UpdateAirportRequest) GetIcaoCode() string {
	if x != nil && x.IcaoCode != nil {
		return *x.IcaoCode
	}
	return ""
}

func (x *UpdateAirportRequest) GetCity() string {
	if x != nil && x.City != nil {
		return *x.City
	}
	return ""
}

func (x *UpdateAirportRequest) GetCountryCode() string {
	if x != nil && x.CountryCode != nil {
		return *x.CountryCode
	}
	return ""
}

func (x *UpdateAirportRequest) GetState() string {
	if x != nil && x.State != nil {
		return *x.State
	}
	return ""
}

func (x *UpdateAirportRequest) GetLat() float64 {
	if x != nil && x.Lat != nil {
		return *x.Lat
	}
	return 0
}

func (x *UpdateAirportRequest) GetLong() float64 {
	if x != nil && x.Long != nil {
		return *x.Long
	}
	return 0
}

func (x *UpdateAirportRequest) GetElevation() string {
	if x != nil && x.Elevation != nil {
		return *x.Elevation
	}
	return ""
}

func (x *UpdateAirportRequest) GetTimezone() string {
	if x != nil && x.Timezone != nil {
		return *x.Timezone
	}
	return ""
}

type UpdateAirportResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Airport *Airport `protobuf:"bytes,1,opt,name=airport,proto3" json:"airport,omitempty"`
}

func (x *UpdateAirportResponse) Reset() {
	*x = UpdateAirportResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_rpc_airport_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateAirportResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateAirportResponse) ProtoMessage() {}

func (x *UpdateAirportResponse) ProtoReflect() protoreflect.Message {
	mi := &file_rpc_airport_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateAirportResponse.ProtoReflect.Descriptor instead.
func (*UpdateAirportResponse) Descriptor() ([]byte, []int) {
	return file_rpc_airport_proto_rawDescGZIP(), []int{8}
}

func (x *UpdateAirportResponse) GetAirport() *Airport {
	if x != nil {
		return x.Airport
	}
	return nil
}

type DeleteAirportRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id       *int64  `protobuf:"varint,1,opt,name=id,proto3,oneof" json:"id,omitempty"`
	Name     *string `protobuf:"bytes,2,opt,name=name,proto3,oneof" json:"name,omitempty"`
	IataCode *string `protobuf:"bytes,3,opt,name=iata_code,json=iataCode,proto3,oneof" json:"iata_code,omitempty"`
	IcaoCode *string `protobuf:"bytes,4,opt,name=icao_code,json=icaoCode,proto3,oneof" json:"icao_code,omitempty"`
}

func (x *DeleteAirportRequest) Reset() {
	*x = DeleteAirportRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_rpc_airport_proto_msgTypes[9]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DeleteAirportRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeleteAirportRequest) ProtoMessage() {}

func (x *DeleteAirportRequest) ProtoReflect() protoreflect.Message {
	mi := &file_rpc_airport_proto_msgTypes[9]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeleteAirportRequest.ProtoReflect.Descriptor instead.
func (*DeleteAirportRequest) Descriptor() ([]byte, []int) {
	return file_rpc_airport_proto_rawDescGZIP(), []int{9}
}

func (x *DeleteAirportRequest) GetId() int64 {
	if x != nil && x.Id != nil {
		return *x.Id
	}
	return 0
}

func (x *DeleteAirportRequest) GetName() string {
	if x != nil && x.Name != nil {
		return *x.Name
	}
	return ""
}

func (x *DeleteAirportRequest) GetIataCode() string {
	if x != nil && x.IataCode != nil {
		return *x.IataCode
	}
	return ""
}

func (x *DeleteAirportRequest) GetIcaoCode() string {
	if x != nil && x.IcaoCode != nil {
		return *x.IcaoCode
	}
	return ""
}

type DeleteAirportResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Airport *Airport `protobuf:"bytes,1,opt,name=airport,proto3" json:"airport,omitempty"`
}

func (x *DeleteAirportResponse) Reset() {
	*x = DeleteAirportResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_rpc_airport_proto_msgTypes[10]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DeleteAirportResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeleteAirportResponse) ProtoMessage() {}

func (x *DeleteAirportResponse) ProtoReflect() protoreflect.Message {
	mi := &file_rpc_airport_proto_msgTypes[10]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeleteAirportResponse.ProtoReflect.Descriptor instead.
func (*DeleteAirportResponse) Descriptor() ([]byte, []int) {
	return file_rpc_airport_proto_rawDescGZIP(), []int{10}
}

func (x *DeleteAirportResponse) GetAirport() *Airport {
	if x != nil {
		return x.Airport
	}
	return nil
}

var File_rpc_airport_proto protoreflect.FileDescriptor

var file_rpc_airport_proto_rawDesc = []byte{
	0x0a, 0x11, 0x72, 0x70, 0x63, 0x5f, 0x61, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x12, 0x02, 0x70, 0x62, 0x22, 0xa4, 0x02, 0x0a, 0x14, 0x43, 0x72, 0x65, 0x61,
	0x74, 0x65, 0x41, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04,
	0x6e, 0x61, 0x6d, 0x65, 0x12, 0x1b, 0x0a, 0x09, 0x69, 0x61, 0x74, 0x61, 0x5f, 0x63, 0x6f, 0x64,
	0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x69, 0x61, 0x74, 0x61, 0x43, 0x6f, 0x64,
	0x65, 0x12, 0x1b, 0x0a, 0x09, 0x69, 0x63, 0x61, 0x6f, 0x5f, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x69, 0x63, 0x61, 0x6f, 0x43, 0x6f, 0x64, 0x65, 0x12, 0x12,
	0x0a, 0x04, 0x63, 0x69, 0x74, 0x79, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x63, 0x69,
	0x74, 0x79, 0x12, 0x21, 0x0a, 0x0c, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x79, 0x5f, 0x63, 0x6f,
	0x64, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x72,
	0x79, 0x43, 0x6f, 0x64, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x73, 0x74, 0x61, 0x74, 0x65, 0x18, 0x06,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x73, 0x74, 0x61, 0x74, 0x65, 0x12, 0x10, 0x0a, 0x03, 0x6c,
	0x61, 0x74, 0x18, 0x07, 0x20, 0x01, 0x28, 0x01, 0x52, 0x03, 0x6c, 0x61, 0x74, 0x12, 0x12, 0x0a,
	0x04, 0x6c, 0x6f, 0x6e, 0x67, 0x18, 0x08, 0x20, 0x01, 0x28, 0x01, 0x52, 0x04, 0x6c, 0x6f, 0x6e,
	0x67, 0x12, 0x21, 0x0a, 0x09, 0x65, 0x6c, 0x65, 0x76, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x09,
	0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x09, 0x65, 0x6c, 0x65, 0x76, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x88, 0x01, 0x01, 0x12, 0x1a, 0x0a, 0x08, 0x74, 0x69, 0x6d, 0x65, 0x7a, 0x6f, 0x6e, 0x65,
	0x18, 0x0a, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x74, 0x69, 0x6d, 0x65, 0x7a, 0x6f, 0x6e, 0x65,
	0x42, 0x0c, 0x0a, 0x0a, 0x5f, 0x65, 0x6c, 0x65, 0x76, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x97,
	0x02, 0x0a, 0x07, 0x41, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61,
	0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x1b,
	0x0a, 0x09, 0x69, 0x61, 0x74, 0x61, 0x5f, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x08, 0x69, 0x61, 0x74, 0x61, 0x43, 0x6f, 0x64, 0x65, 0x12, 0x1b, 0x0a, 0x09, 0x69,
	0x63, 0x61, 0x6f, 0x5f, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08,
	0x69, 0x63, 0x61, 0x6f, 0x43, 0x6f, 0x64, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x63, 0x69, 0x74, 0x79,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x63, 0x69, 0x74, 0x79, 0x12, 0x21, 0x0a, 0x0c,
	0x63, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x79, 0x5f, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x05, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x0b, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x79, 0x43, 0x6f, 0x64, 0x65, 0x12,
	0x14, 0x0a, 0x05, 0x73, 0x74, 0x61, 0x74, 0x65, 0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05,
	0x73, 0x74, 0x61, 0x74, 0x65, 0x12, 0x10, 0x0a, 0x03, 0x6c, 0x61, 0x74, 0x18, 0x07, 0x20, 0x01,
	0x28, 0x01, 0x52, 0x03, 0x6c, 0x61, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x6c, 0x6f, 0x6e, 0x67, 0x18,
	0x08, 0x20, 0x01, 0x28, 0x01, 0x52, 0x04, 0x6c, 0x6f, 0x6e, 0x67, 0x12, 0x21, 0x0a, 0x09, 0x65,
	0x6c, 0x65, 0x76, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x09, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00,
	0x52, 0x09, 0x65, 0x6c, 0x65, 0x76, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x88, 0x01, 0x01, 0x12, 0x1a,
	0x0a, 0x08, 0x74, 0x69, 0x6d, 0x65, 0x7a, 0x6f, 0x6e, 0x65, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x08, 0x74, 0x69, 0x6d, 0x65, 0x7a, 0x6f, 0x6e, 0x65, 0x42, 0x0c, 0x0a, 0x0a, 0x5f, 0x65,
	0x6c, 0x65, 0x76, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x3e, 0x0a, 0x15, 0x43, 0x72, 0x65, 0x61,
	0x74, 0x65, 0x41, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x25, 0x0a, 0x07, 0x61, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x0b, 0x2e, 0x70, 0x62, 0x2e, 0x41, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x52,
	0x07, 0x61, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x22, 0x3c, 0x0a, 0x13, 0x4c, 0x69, 0x73, 0x74,
	0x41, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12,
	0x25, 0x0a, 0x07, 0x61, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b,
	0x32, 0x0b, 0x2e, 0x70, 0x62, 0x2e, 0x41, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x52, 0x07, 0x61,
	0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x22, 0x42, 0x0a, 0x12, 0x4c, 0x69, 0x73, 0x74, 0x41, 0x69,
	0x72, 0x70, 0x6f, 0x72, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x14, 0x0a, 0x05,
	0x6c, 0x69, 0x6d, 0x69, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x05, 0x6c, 0x69, 0x6d,
	0x69, 0x74, 0x12, 0x16, 0x0a, 0x06, 0x6f, 0x66, 0x66, 0x73, 0x65, 0x74, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x03, 0x52, 0x06, 0x6f, 0x66, 0x66, 0x73, 0x65, 0x74, 0x22, 0xa5, 0x01, 0x0a, 0x11, 0x47,
	0x65, 0x74, 0x41, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x02, 0x69, 0x64,
	0x12, 0x17, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00,
	0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x88, 0x01, 0x01, 0x12, 0x20, 0x0a, 0x09, 0x69, 0x61, 0x74,
	0x61, 0x5f, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x48, 0x01, 0x52, 0x08,
	0x69, 0x61, 0x74, 0x61, 0x43, 0x6f, 0x64, 0x65, 0x88, 0x01, 0x01, 0x12, 0x20, 0x0a, 0x09, 0x69,
	0x63, 0x61, 0x6f, 0x5f, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x48, 0x02,
	0x52, 0x08, 0x69, 0x63, 0x61, 0x6f, 0x43, 0x6f, 0x64, 0x65, 0x88, 0x01, 0x01, 0x42, 0x07, 0x0a,
	0x05, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x42, 0x0c, 0x0a, 0x0a, 0x5f, 0x69, 0x61, 0x74, 0x61, 0x5f,
	0x63, 0x6f, 0x64, 0x65, 0x42, 0x0c, 0x0a, 0x0a, 0x5f, 0x69, 0x63, 0x61, 0x6f, 0x5f, 0x63, 0x6f,
	0x64, 0x65, 0x22, 0x3b, 0x0a, 0x12, 0x47, 0x65, 0x74, 0x41, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x25, 0x0a, 0x07, 0x61, 0x69, 0x72, 0x70,
	0x6f, 0x72, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0b, 0x2e, 0x70, 0x62, 0x2e, 0x41,
	0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x52, 0x07, 0x61, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x22,
	0xc8, 0x03, 0x0a, 0x14, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x41, 0x69, 0x72, 0x70, 0x6f, 0x72,
	0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x03, 0x52, 0x02, 0x69, 0x64, 0x12, 0x17, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x88, 0x01,
	0x01, 0x12, 0x20, 0x0a, 0x09, 0x69, 0x61, 0x74, 0x61, 0x5f, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x09, 0x48, 0x01, 0x52, 0x08, 0x69, 0x61, 0x74, 0x61, 0x43, 0x6f, 0x64, 0x65,
	0x88, 0x01, 0x01, 0x12, 0x20, 0x0a, 0x09, 0x69, 0x63, 0x61, 0x6f, 0x5f, 0x63, 0x6f, 0x64, 0x65,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x48, 0x02, 0x52, 0x08, 0x69, 0x63, 0x61, 0x6f, 0x43, 0x6f,
	0x64, 0x65, 0x88, 0x01, 0x01, 0x12, 0x17, 0x0a, 0x04, 0x63, 0x69, 0x74, 0x79, 0x18, 0x05, 0x20,
	0x01, 0x28, 0x09, 0x48, 0x03, 0x52, 0x04, 0x63, 0x69, 0x74, 0x79, 0x88, 0x01, 0x01, 0x12, 0x26,
	0x0a, 0x0c, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x79, 0x5f, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x06,
	0x20, 0x01, 0x28, 0x09, 0x48, 0x04, 0x52, 0x0b, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x79, 0x43,
	0x6f, 0x64, 0x65, 0x88, 0x01, 0x01, 0x12, 0x19, 0x0a, 0x05, 0x73, 0x74, 0x61, 0x74, 0x65, 0x18,
	0x07, 0x20, 0x01, 0x28, 0x09, 0x48, 0x05, 0x52, 0x05, 0x73, 0x74, 0x61, 0x74, 0x65, 0x88, 0x01,
	0x01, 0x12, 0x15, 0x0a, 0x03, 0x6c, 0x61, 0x74, 0x18, 0x08, 0x20, 0x01, 0x28, 0x01, 0x48, 0x06,
	0x52, 0x03, 0x6c, 0x61, 0x74, 0x88, 0x01, 0x01, 0x12, 0x17, 0x0a, 0x04, 0x6c, 0x6f, 0x6e, 0x67,
	0x18, 0x09, 0x20, 0x01, 0x28, 0x01, 0x48, 0x07, 0x52, 0x04, 0x6c, 0x6f, 0x6e, 0x67, 0x88, 0x01,
	0x01, 0x12, 0x21, 0x0a, 0x09, 0x65, 0x6c, 0x65, 0x76, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x0a,
	0x20, 0x01, 0x28, 0x09, 0x48, 0x08, 0x52, 0x09, 0x65, 0x6c, 0x65, 0x76, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x88, 0x01, 0x01, 0x12, 0x1f, 0x0a, 0x08, 0x74, 0x69, 0x6d, 0x65, 0x7a, 0x6f, 0x6e, 0x65,
	0x18, 0x0b, 0x20, 0x01, 0x28, 0x09, 0x48, 0x09, 0x52, 0x08, 0x74, 0x69, 0x6d, 0x65, 0x7a, 0x6f,
	0x6e, 0x65, 0x88, 0x01, 0x01, 0x42, 0x07, 0x0a, 0x05, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x42, 0x0c,
	0x0a, 0x0a, 0x5f, 0x69, 0x61, 0x74, 0x61, 0x5f, 0x63, 0x6f, 0x64, 0x65, 0x42, 0x0c, 0x0a, 0x0a,
	0x5f, 0x69, 0x63, 0x61, 0x6f, 0x5f, 0x63, 0x6f, 0x64, 0x65, 0x42, 0x07, 0x0a, 0x05, 0x5f, 0x63,
	0x69, 0x74, 0x79, 0x42, 0x0f, 0x0a, 0x0d, 0x5f, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x72, 0x79, 0x5f,
	0x63, 0x6f, 0x64, 0x65, 0x42, 0x08, 0x0a, 0x06, 0x5f, 0x73, 0x74, 0x61, 0x74, 0x65, 0x42, 0x06,
	0x0a, 0x04, 0x5f, 0x6c, 0x61, 0x74, 0x42, 0x07, 0x0a, 0x05, 0x5f, 0x6c, 0x6f, 0x6e, 0x67, 0x42,
	0x0c, 0x0a, 0x0a, 0x5f, 0x65, 0x6c, 0x65, 0x76, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x42, 0x0b, 0x0a,
	0x09, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x7a, 0x6f, 0x6e, 0x65, 0x22, 0x3e, 0x0a, 0x15, 0x55, 0x70,
	0x64, 0x61, 0x74, 0x65, 0x41, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x12, 0x25, 0x0a, 0x07, 0x61, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x0b, 0x2e, 0x70, 0x62, 0x2e, 0x41, 0x69, 0x72, 0x70, 0x6f, 0x72,
	0x74, 0x52, 0x07, 0x61, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x22, 0xb4, 0x01, 0x0a, 0x14, 0x44,
	0x65, 0x6c, 0x65, 0x74, 0x65, 0x41, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x12, 0x13, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x48,
	0x00, 0x52, 0x02, 0x69, 0x64, 0x88, 0x01, 0x01, 0x12, 0x17, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x48, 0x01, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x88, 0x01,
	0x01, 0x12, 0x20, 0x0a, 0x09, 0x69, 0x61, 0x74, 0x61, 0x5f, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x09, 0x48, 0x02, 0x52, 0x08, 0x69, 0x61, 0x74, 0x61, 0x43, 0x6f, 0x64, 0x65,
	0x88, 0x01, 0x01, 0x12, 0x20, 0x0a, 0x09, 0x69, 0x63, 0x61, 0x6f, 0x5f, 0x63, 0x6f, 0x64, 0x65,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x48, 0x03, 0x52, 0x08, 0x69, 0x63, 0x61, 0x6f, 0x43, 0x6f,
	0x64, 0x65, 0x88, 0x01, 0x01, 0x42, 0x05, 0x0a, 0x03, 0x5f, 0x69, 0x64, 0x42, 0x07, 0x0a, 0x05,
	0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x42, 0x0c, 0x0a, 0x0a, 0x5f, 0x69, 0x61, 0x74, 0x61, 0x5f, 0x63,
	0x6f, 0x64, 0x65, 0x42, 0x0c, 0x0a, 0x0a, 0x5f, 0x69, 0x63, 0x61, 0x6f, 0x5f, 0x63, 0x6f, 0x64,
	0x65, 0x22, 0x3e, 0x0a, 0x15, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x41, 0x69, 0x72, 0x70, 0x6f,
	0x72, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x25, 0x0a, 0x07, 0x61, 0x69,
	0x72, 0x70, 0x6f, 0x72, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0b, 0x2e, 0x70, 0x62,
	0x2e, 0x41, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x52, 0x07, 0x61, 0x69, 0x72, 0x70, 0x6f, 0x72,
	0x74, 0x42, 0x28, 0x5a, 0x26, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x6e, 0x69, 0x69, 0x6e, 0x69, 0x79, 0x61, 0x72, 0x65, 0x2f, 0x61, 0x77, 0x6f, 0x2f, 0x70, 0x6b,
	0x67, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x76, 0x31, 0x2f, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x33,
}

var (
	file_rpc_airport_proto_rawDescOnce sync.Once
	file_rpc_airport_proto_rawDescData = file_rpc_airport_proto_rawDesc
)

func file_rpc_airport_proto_rawDescGZIP() []byte {
	file_rpc_airport_proto_rawDescOnce.Do(func() {
		file_rpc_airport_proto_rawDescData = protoimpl.X.CompressGZIP(file_rpc_airport_proto_rawDescData)
	})
	return file_rpc_airport_proto_rawDescData
}

var file_rpc_airport_proto_msgTypes = make([]protoimpl.MessageInfo, 11)
var file_rpc_airport_proto_goTypes = []interface{}{
	(*CreateAirportRequest)(nil),  // 0: pb.CreateAirportRequest
	(*Airport)(nil),               // 1: pb.Airport
	(*CreateAirportResponse)(nil), // 2: pb.CreateAirportResponse
	(*ListAirportResponse)(nil),   // 3: pb.ListAirportResponse
	(*ListAirportRequest)(nil),    // 4: pb.ListAirportRequest
	(*GetAirportRequest)(nil),     // 5: pb.GetAirportRequest
	(*GetAirportResponse)(nil),    // 6: pb.GetAirportResponse
	(*UpdateAirportRequest)(nil),  // 7: pb.UpdateAirportRequest
	(*UpdateAirportResponse)(nil), // 8: pb.UpdateAirportResponse
	(*DeleteAirportRequest)(nil),  // 9: pb.DeleteAirportRequest
	(*DeleteAirportResponse)(nil), // 10: pb.DeleteAirportResponse
}
var file_rpc_airport_proto_depIdxs = []int32{
	1, // 0: pb.CreateAirportResponse.airport:type_name -> pb.Airport
	1, // 1: pb.ListAirportResponse.airport:type_name -> pb.Airport
	1, // 2: pb.GetAirportResponse.airport:type_name -> pb.Airport
	1, // 3: pb.UpdateAirportResponse.airport:type_name -> pb.Airport
	1, // 4: pb.DeleteAirportResponse.airport:type_name -> pb.Airport
	5, // [5:5] is the sub-list for method output_type
	5, // [5:5] is the sub-list for method input_type
	5, // [5:5] is the sub-list for extension type_name
	5, // [5:5] is the sub-list for extension extendee
	0, // [0:5] is the sub-list for field type_name
}

func init() { file_rpc_airport_proto_init() }
func file_rpc_airport_proto_init() {
	if File_rpc_airport_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_rpc_airport_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CreateAirportRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_rpc_airport_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Airport); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_rpc_airport_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CreateAirportResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_rpc_airport_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListAirportResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_rpc_airport_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListAirportRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_rpc_airport_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetAirportRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_rpc_airport_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetAirportResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_rpc_airport_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UpdateAirportRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_rpc_airport_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UpdateAirportResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_rpc_airport_proto_msgTypes[9].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DeleteAirportRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_rpc_airport_proto_msgTypes[10].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DeleteAirportResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	file_rpc_airport_proto_msgTypes[0].OneofWrappers = []interface{}{}
	file_rpc_airport_proto_msgTypes[1].OneofWrappers = []interface{}{}
	file_rpc_airport_proto_msgTypes[5].OneofWrappers = []interface{}{}
	file_rpc_airport_proto_msgTypes[7].OneofWrappers = []interface{}{}
	file_rpc_airport_proto_msgTypes[9].OneofWrappers = []interface{}{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_rpc_airport_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   11,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_rpc_airport_proto_goTypes,
		DependencyIndexes: file_rpc_airport_proto_depIdxs,
		MessageInfos:      file_rpc_airport_proto_msgTypes,
	}.Build()
	File_rpc_airport_proto = out.File
	file_rpc_airport_proto_rawDesc = nil
	file_rpc_airport_proto_goTypes = nil
	file_rpc_airport_proto_depIdxs = nil
}
