// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.19.0
// source: flight_msg.proto

package flightinfo

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Flight struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	FlightId            int32                  `protobuf:"varint,1,opt,name=flight_id,json=flightId,proto3" json:"flight_id,omitempty"`
	FlightNo            int64                  `protobuf:"varint,2,opt,name=flight_no,json=flightNo,proto3" json:"flight_no,omitempty"`
	ScheduledDeparture  *timestamppb.Timestamp `protobuf:"bytes,3,opt,name=scheduled_departure,json=scheduledDeparture,proto3" json:"scheduled_departure,omitempty"`
	ScheduledArrival    *timestamppb.Timestamp `protobuf:"bytes,4,opt,name=scheduled_arrival,json=scheduledArrival,proto3" json:"scheduled_arrival,omitempty"`
	DepartureAirport    *Airport               `protobuf:"bytes,5,opt,name=departure_airport,json=departureAirport,proto3" json:"departure_airport,omitempty"`
	ArrivalAirport      *Airport               `protobuf:"bytes,6,opt,name=arrival_airport,json=arrivalAirport,proto3" json:"arrival_airport,omitempty"`
	Status              string                 `protobuf:"bytes,7,opt,name=status,proto3" json:"status,omitempty"`
	AircraftCode        string                 `protobuf:"bytes,8,opt,name=aircraft_code,json=aircraftCode,proto3" json:"aircraft_code,omitempty"`
	ActualDepartureTime *timestamppb.Timestamp `protobuf:"bytes,9,opt,name=actual_departure_time,json=actualDepartureTime,proto3" json:"actual_departure_time,omitempty"`
	ActualArrivalTime   *timestamppb.Timestamp `protobuf:"bytes,10,opt,name=actual_arrival_time,json=actualArrivalTime,proto3" json:"actual_arrival_time,omitempty"`
}

func (x *Flight) Reset() {
	*x = Flight{}
	if protoimpl.UnsafeEnabled {
		mi := &file_flight_msg_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Flight) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Flight) ProtoMessage() {}

func (x *Flight) ProtoReflect() protoreflect.Message {
	mi := &file_flight_msg_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Flight.ProtoReflect.Descriptor instead.
func (*Flight) Descriptor() ([]byte, []int) {
	return file_flight_msg_proto_rawDescGZIP(), []int{0}
}

func (x *Flight) GetFlightId() int32 {
	if x != nil {
		return x.FlightId
	}
	return 0
}

func (x *Flight) GetFlightNo() int64 {
	if x != nil {
		return x.FlightNo
	}
	return 0
}

func (x *Flight) GetScheduledDeparture() *timestamppb.Timestamp {
	if x != nil {
		return x.ScheduledDeparture
	}
	return nil
}

func (x *Flight) GetScheduledArrival() *timestamppb.Timestamp {
	if x != nil {
		return x.ScheduledArrival
	}
	return nil
}

func (x *Flight) GetDepartureAirport() *Airport {
	if x != nil {
		return x.DepartureAirport
	}
	return nil
}

func (x *Flight) GetArrivalAirport() *Airport {
	if x != nil {
		return x.ArrivalAirport
	}
	return nil
}

func (x *Flight) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

func (x *Flight) GetAircraftCode() string {
	if x != nil {
		return x.AircraftCode
	}
	return ""
}

func (x *Flight) GetActualDepartureTime() *timestamppb.Timestamp {
	if x != nil {
		return x.ActualDepartureTime
	}
	return nil
}

func (x *Flight) GetActualArrivalTime() *timestamppb.Timestamp {
	if x != nil {
		return x.ActualArrivalTime
	}
	return nil
}

type FlightRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	DepartureAirport   *Airport               `protobuf:"bytes,1,opt,name=departure_airport,json=departureAirport,proto3" json:"departure_airport,omitempty"`
	ArrivalAirport     *Airport               `protobuf:"bytes,2,opt,name=arrival_airport,json=arrivalAirport,proto3" json:"arrival_airport,omitempty"`
	ScheduledDeparture *timestamppb.Timestamp `protobuf:"bytes,3,opt,name=scheduled_departure,json=scheduledDeparture,proto3" json:"scheduled_departure,omitempty"`
	ScheduledArrival   *timestamppb.Timestamp `protobuf:"bytes,4,opt,name=scheduled_arrival,json=scheduledArrival,proto3" json:"scheduled_arrival,omitempty"`
}

func (x *FlightRequest) Reset() {
	*x = FlightRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_flight_msg_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *FlightRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FlightRequest) ProtoMessage() {}

func (x *FlightRequest) ProtoReflect() protoreflect.Message {
	mi := &file_flight_msg_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FlightRequest.ProtoReflect.Descriptor instead.
func (*FlightRequest) Descriptor() ([]byte, []int) {
	return file_flight_msg_proto_rawDescGZIP(), []int{1}
}

func (x *FlightRequest) GetDepartureAirport() *Airport {
	if x != nil {
		return x.DepartureAirport
	}
	return nil
}

func (x *FlightRequest) GetArrivalAirport() *Airport {
	if x != nil {
		return x.ArrivalAirport
	}
	return nil
}

func (x *FlightRequest) GetScheduledDeparture() *timestamppb.Timestamp {
	if x != nil {
		return x.ScheduledDeparture
	}
	return nil
}

func (x *FlightRequest) GetScheduledArrival() *timestamppb.Timestamp {
	if x != nil {
		return x.ScheduledArrival
	}
	return nil
}

type FlightResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Flight []*Flight `protobuf:"bytes,1,rep,name=flight,proto3" json:"flight,omitempty"`
}

func (x *FlightResponse) Reset() {
	*x = FlightResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_flight_msg_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *FlightResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FlightResponse) ProtoMessage() {}

func (x *FlightResponse) ProtoReflect() protoreflect.Message {
	mi := &file_flight_msg_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FlightResponse.ProtoReflect.Descriptor instead.
func (*FlightResponse) Descriptor() ([]byte, []int) {
	return file_flight_msg_proto_rawDescGZIP(), []int{2}
}

func (x *FlightResponse) GetFlight() []*Flight {
	if x != nil {
		return x.Flight
	}
	return nil
}

var File_flight_msg_proto protoreflect.FileDescriptor

var file_flight_msg_proto_rawDesc = []byte{
	0x0a, 0x10, 0x66, 0x6c, 0x69, 0x67, 0x68, 0x74, 0x5f, 0x6d, 0x73, 0x67, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x12, 0x0a, 0x66, 0x6c, 0x69, 0x67, 0x68, 0x74, 0x69, 0x6e, 0x66, 0x6f, 0x1a, 0x1f,
	0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f,
	0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a,
	0x11, 0x61, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x5f, 0x6d, 0x73, 0x67, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x22, 0xb1, 0x04, 0x0a, 0x06, 0x46, 0x6c, 0x69, 0x67, 0x68, 0x74, 0x12, 0x1b, 0x0a,
	0x09, 0x66, 0x6c, 0x69, 0x67, 0x68, 0x74, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05,
	0x52, 0x08, 0x66, 0x6c, 0x69, 0x67, 0x68, 0x74, 0x49, 0x64, 0x12, 0x1b, 0x0a, 0x09, 0x66, 0x6c,
	0x69, 0x67, 0x68, 0x74, 0x5f, 0x6e, 0x6f, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x08, 0x66,
	0x6c, 0x69, 0x67, 0x68, 0x74, 0x4e, 0x6f, 0x12, 0x4b, 0x0a, 0x13, 0x73, 0x63, 0x68, 0x65, 0x64,
	0x75, 0x6c, 0x65, 0x64, 0x5f, 0x64, 0x65, 0x70, 0x61, 0x72, 0x74, 0x75, 0x72, 0x65, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70,
	0x52, 0x12, 0x73, 0x63, 0x68, 0x65, 0x64, 0x75, 0x6c, 0x65, 0x64, 0x44, 0x65, 0x70, 0x61, 0x72,
	0x74, 0x75, 0x72, 0x65, 0x12, 0x47, 0x0a, 0x11, 0x73, 0x63, 0x68, 0x65, 0x64, 0x75, 0x6c, 0x65,
	0x64, 0x5f, 0x61, 0x72, 0x72, 0x69, 0x76, 0x61, 0x6c, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75,
	0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x10, 0x73, 0x63, 0x68,
	0x65, 0x64, 0x75, 0x6c, 0x65, 0x64, 0x41, 0x72, 0x72, 0x69, 0x76, 0x61, 0x6c, 0x12, 0x40, 0x0a,
	0x11, 0x64, 0x65, 0x70, 0x61, 0x72, 0x74, 0x75, 0x72, 0x65, 0x5f, 0x61, 0x69, 0x72, 0x70, 0x6f,
	0x72, 0x74, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x13, 0x2e, 0x66, 0x6c, 0x69, 0x67, 0x68,
	0x74, 0x69, 0x6e, 0x66, 0x6f, 0x2e, 0x41, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x52, 0x10, 0x64,
	0x65, 0x70, 0x61, 0x72, 0x74, 0x75, 0x72, 0x65, 0x41, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x12,
	0x3c, 0x0a, 0x0f, 0x61, 0x72, 0x72, 0x69, 0x76, 0x61, 0x6c, 0x5f, 0x61, 0x69, 0x72, 0x70, 0x6f,
	0x72, 0x74, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x13, 0x2e, 0x66, 0x6c, 0x69, 0x67, 0x68,
	0x74, 0x69, 0x6e, 0x66, 0x6f, 0x2e, 0x41, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x52, 0x0e, 0x61,
	0x72, 0x72, 0x69, 0x76, 0x61, 0x6c, 0x41, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x12, 0x16, 0x0a,
	0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x07, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x73,
	0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x23, 0x0a, 0x0d, 0x61, 0x69, 0x72, 0x63, 0x72, 0x61, 0x66,
	0x74, 0x5f, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x08, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0c, 0x61, 0x69,
	0x72, 0x63, 0x72, 0x61, 0x66, 0x74, 0x43, 0x6f, 0x64, 0x65, 0x12, 0x4e, 0x0a, 0x15, 0x61, 0x63,
	0x74, 0x75, 0x61, 0x6c, 0x5f, 0x64, 0x65, 0x70, 0x61, 0x72, 0x74, 0x75, 0x72, 0x65, 0x5f, 0x74,
	0x69, 0x6d, 0x65, 0x18, 0x09, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67,
	0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65,
	0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x13, 0x61, 0x63, 0x74, 0x75, 0x61, 0x6c, 0x44, 0x65, 0x70,
	0x61, 0x72, 0x74, 0x75, 0x72, 0x65, 0x54, 0x69, 0x6d, 0x65, 0x12, 0x4a, 0x0a, 0x13, 0x61, 0x63,
	0x74, 0x75, 0x61, 0x6c, 0x5f, 0x61, 0x72, 0x72, 0x69, 0x76, 0x61, 0x6c, 0x5f, 0x74, 0x69, 0x6d,
	0x65, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74,
	0x61, 0x6d, 0x70, 0x52, 0x11, 0x61, 0x63, 0x74, 0x75, 0x61, 0x6c, 0x41, 0x72, 0x72, 0x69, 0x76,
	0x61, 0x6c, 0x54, 0x69, 0x6d, 0x65, 0x22, 0xa5, 0x02, 0x0a, 0x0d, 0x46, 0x6c, 0x69, 0x67, 0x68,
	0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x40, 0x0a, 0x11, 0x64, 0x65, 0x70, 0x61,
	0x72, 0x74, 0x75, 0x72, 0x65, 0x5f, 0x61, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x13, 0x2e, 0x66, 0x6c, 0x69, 0x67, 0x68, 0x74, 0x69, 0x6e, 0x66, 0x6f,
	0x2e, 0x41, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x52, 0x10, 0x64, 0x65, 0x70, 0x61, 0x72, 0x74,
	0x75, 0x72, 0x65, 0x41, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x12, 0x3c, 0x0a, 0x0f, 0x61, 0x72,
	0x72, 0x69, 0x76, 0x61, 0x6c, 0x5f, 0x61, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x13, 0x2e, 0x66, 0x6c, 0x69, 0x67, 0x68, 0x74, 0x69, 0x6e, 0x66, 0x6f,
	0x2e, 0x41, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x52, 0x0e, 0x61, 0x72, 0x72, 0x69, 0x76, 0x61,
	0x6c, 0x41, 0x69, 0x72, 0x70, 0x6f, 0x72, 0x74, 0x12, 0x4b, 0x0a, 0x13, 0x73, 0x63, 0x68, 0x65,
	0x64, 0x75, 0x6c, 0x65, 0x64, 0x5f, 0x64, 0x65, 0x70, 0x61, 0x72, 0x74, 0x75, 0x72, 0x65, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d,
	0x70, 0x52, 0x12, 0x73, 0x63, 0x68, 0x65, 0x64, 0x75, 0x6c, 0x65, 0x64, 0x44, 0x65, 0x70, 0x61,
	0x72, 0x74, 0x75, 0x72, 0x65, 0x12, 0x47, 0x0a, 0x11, 0x73, 0x63, 0x68, 0x65, 0x64, 0x75, 0x6c,
	0x65, 0x64, 0x5f, 0x61, 0x72, 0x72, 0x69, 0x76, 0x61, 0x6c, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x10, 0x73, 0x63,
	0x68, 0x65, 0x64, 0x75, 0x6c, 0x65, 0x64, 0x41, 0x72, 0x72, 0x69, 0x76, 0x61, 0x6c, 0x22, 0x3c,
	0x0a, 0x0e, 0x46, 0x6c, 0x69, 0x67, 0x68, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x12, 0x2a, 0x0a, 0x06, 0x66, 0x6c, 0x69, 0x67, 0x68, 0x74, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b,
	0x32, 0x12, 0x2e, 0x66, 0x6c, 0x69, 0x67, 0x68, 0x74, 0x69, 0x6e, 0x66, 0x6f, 0x2e, 0x46, 0x6c,
	0x69, 0x67, 0x68, 0x74, 0x52, 0x06, 0x66, 0x6c, 0x69, 0x67, 0x68, 0x74, 0x32, 0x92, 0x01, 0x0a,
	0x0d, 0x46, 0x6c, 0x69, 0x67, 0x68, 0x74, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x38,
	0x0a, 0x0c, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x46, 0x6c, 0x69, 0x67, 0x68, 0x74, 0x12, 0x12,
	0x2e, 0x66, 0x6c, 0x69, 0x67, 0x68, 0x74, 0x69, 0x6e, 0x66, 0x6f, 0x2e, 0x46, 0x6c, 0x69, 0x67,
	0x68, 0x74, 0x1a, 0x12, 0x2e, 0x66, 0x6c, 0x69, 0x67, 0x68, 0x74, 0x69, 0x6e, 0x66, 0x6f, 0x2e,
	0x46, 0x6c, 0x69, 0x67, 0x68, 0x74, 0x22, 0x00, 0x12, 0x47, 0x0a, 0x0c, 0x53, 0x65, 0x61, 0x72,
	0x63, 0x68, 0x46, 0x6c, 0x69, 0x67, 0x68, 0x74, 0x12, 0x19, 0x2e, 0x66, 0x6c, 0x69, 0x67, 0x68,
	0x74, 0x69, 0x6e, 0x66, 0x6f, 0x2e, 0x46, 0x6c, 0x69, 0x67, 0x68, 0x74, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x1a, 0x1a, 0x2e, 0x66, 0x6c, 0x69, 0x67, 0x68, 0x74, 0x69, 0x6e, 0x66, 0x6f,
	0x2e, 0x46, 0x6c, 0x69, 0x67, 0x68, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22,
	0x00, 0x42, 0x11, 0x5a, 0x0f, 0x2e, 0x2f, 0x70, 0x62, 0x3b, 0x66, 0x6c, 0x69, 0x67, 0x68, 0x74,
	0x69, 0x6e, 0x66, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_flight_msg_proto_rawDescOnce sync.Once
	file_flight_msg_proto_rawDescData = file_flight_msg_proto_rawDesc
)

func file_flight_msg_proto_rawDescGZIP() []byte {
	file_flight_msg_proto_rawDescOnce.Do(func() {
		file_flight_msg_proto_rawDescData = protoimpl.X.CompressGZIP(file_flight_msg_proto_rawDescData)
	})
	return file_flight_msg_proto_rawDescData
}

var file_flight_msg_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_flight_msg_proto_goTypes = []interface{}{
	(*Flight)(nil),                // 0: flightinfo.Flight
	(*FlightRequest)(nil),         // 1: flightinfo.FlightRequest
	(*FlightResponse)(nil),        // 2: flightinfo.FlightResponse
	(*timestamppb.Timestamp)(nil), // 3: google.protobuf.Timestamp
	(*Airport)(nil),               // 4: flightinfo.Airport
}
var file_flight_msg_proto_depIdxs = []int32{
	3,  // 0: flightinfo.Flight.scheduled_departure:type_name -> google.protobuf.Timestamp
	3,  // 1: flightinfo.Flight.scheduled_arrival:type_name -> google.protobuf.Timestamp
	4,  // 2: flightinfo.Flight.departure_airport:type_name -> flightinfo.Airport
	4,  // 3: flightinfo.Flight.arrival_airport:type_name -> flightinfo.Airport
	3,  // 4: flightinfo.Flight.actual_departure_time:type_name -> google.protobuf.Timestamp
	3,  // 5: flightinfo.Flight.actual_arrival_time:type_name -> google.protobuf.Timestamp
	4,  // 6: flightinfo.FlightRequest.departure_airport:type_name -> flightinfo.Airport
	4,  // 7: flightinfo.FlightRequest.arrival_airport:type_name -> flightinfo.Airport
	3,  // 8: flightinfo.FlightRequest.scheduled_departure:type_name -> google.protobuf.Timestamp
	3,  // 9: flightinfo.FlightRequest.scheduled_arrival:type_name -> google.protobuf.Timestamp
	0,  // 10: flightinfo.FlightResponse.flight:type_name -> flightinfo.Flight
	0,  // 11: flightinfo.FlightService.CreateFlight:input_type -> flightinfo.Flight
	1,  // 12: flightinfo.FlightService.SearchFlight:input_type -> flightinfo.FlightRequest
	0,  // 13: flightinfo.FlightService.CreateFlight:output_type -> flightinfo.Flight
	2,  // 14: flightinfo.FlightService.SearchFlight:output_type -> flightinfo.FlightResponse
	13, // [13:15] is the sub-list for method output_type
	11, // [11:13] is the sub-list for method input_type
	11, // [11:11] is the sub-list for extension type_name
	11, // [11:11] is the sub-list for extension extendee
	0,  // [0:11] is the sub-list for field type_name
}

func init() { file_flight_msg_proto_init() }
func file_flight_msg_proto_init() {
	if File_flight_msg_proto != nil {
		return
	}
	file_airport_msg_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_flight_msg_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Flight); i {
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
		file_flight_msg_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*FlightRequest); i {
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
		file_flight_msg_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*FlightResponse); i {
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
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_flight_msg_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_flight_msg_proto_goTypes,
		DependencyIndexes: file_flight_msg_proto_depIdxs,
		MessageInfos:      file_flight_msg_proto_msgTypes,
	}.Build()
	File_flight_msg_proto = out.File
	file_flight_msg_proto_rawDesc = nil
	file_flight_msg_proto_goTypes = nil
	file_flight_msg_proto_depIdxs = nil
}
