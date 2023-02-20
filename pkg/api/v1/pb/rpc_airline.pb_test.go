// Generated TestCreateAirlineRequest_Reset
// Generated TestCreateAirlineRequest_String
// Generated TestCreateAirlineRequest_ProtoMessage
// Generated TestCreateAirlineRequest_ProtoReflect
// Generated TestCreateAirlineRequest_Descriptor
// Generated TestCreateAirlineRequest_GetName
// Generated TestCreateAirlineRequest_GetIataCode
// Generated TestCreateAirlineRequest_GetIcaoCode
// Generated TestCreateAirlineRequest_GetCountryCode
// Generated TestLinks_Reset
// Generated TestLinks_String
// Generated TestLinks_ProtoMessage
// Generated TestLinks_ProtoReflect
// Generated TestLinks_Descriptor
// Generated TestLinks_GetWebsite
// Generated TestLinks_GetTwitter
// Generated TestLinks_GetInstagram
// Generated TestLinks_GetLinkedin
// Generated TestAirline_Reset
// Generated TestAirline_String
// Generated TestAirline_ProtoMessage
// Generated TestAirline_ProtoReflect
// Generated TestAirline_Descriptor
// Generated TestAirline_GetId
// Generated TestAirline_GetName
// Generated TestAirline_GetIataCode
// Generated TestAirline_GetIcaoCode
// Generated TestAirline_GetCountryCode
// Generated TestAirline_GetCreated_At
// Generated TestAirline_GetUpdated_At
// Generated TestCreateAirlineResponse_Reset
// Generated TestCreateAirlineResponse_String
// Generated TestCreateAirlineResponse_ProtoMessage
// Generated TestCreateAirlineResponse_ProtoReflect
// Generated TestCreateAirlineResponse_Descriptor
// Generated TestCreateAirlineResponse_GetId
// Generated TestCreateAirlineResponse_GetName
// Generated TestCreateAirlineResponse_GetIataCode
// Generated TestCreateAirlineResponse_GetIcaoCode
// Generated TestCreateAirlineResponse_GetCountryCode
// Generated TestCreateAirlineResponse_GetCreated_At
// Generated TestCreateAirlineResponse_GetUpdated_At
// Generated TestListAirlineRes_Reset
// Generated TestListAirlineRes_String
// Generated TestListAirlineRes_ProtoMessage
// Generated TestListAirlineRes_ProtoReflect
// Generated TestListAirlineRes_Descriptor
// Generated TestListAirlineRes_GetAirlines
// Generated TestListAirlinesReq_Reset
// Generated TestListAirlinesReq_String
// Generated TestListAirlinesReq_ProtoMessage
// Generated TestListAirlinesReq_ProtoReflect
// Generated TestListAirlinesReq_Descriptor
// Generated TestListAirlinesReq_GetLimit
// Generated TestListAirlinesReq_GetOffset
// Generated TestGetAirlineRequest_Reset
// Generated TestGetAirlineRequest_String
// Generated TestGetAirlineRequest_ProtoMessage
// Generated TestGetAirlineRequest_ProtoReflect
// Generated TestGetAirlineRequest_Descriptor
// Generated TestGetAirlineRequest_GetId
// Generated TestGetAirlineRequest_GetName
// Generated TestGetAirlineRequest_GetIataCode
// Generated TestGetAirlineRequest_GetIcaoCode
// Generated TestGetAirlineResponse_Reset
// Generated TestGetAirlineResponse_String
// Generated TestGetAirlineResponse_ProtoMessage
// Generated TestGetAirlineResponse_ProtoReflect
// Generated TestGetAirlineResponse_Descriptor
// Generated TestGetAirlineResponse_GetId
// Generated TestGetAirlineResponse_GetName
// Generated TestGetAirlineResponse_GetIataCode
// Generated TestGetAirlineResponse_GetIcaoCode
// Generated TestGetAirlineResponse_GetCountryCode
// Generated TestGetAirlineResponse_GetCreated_At
// Generated TestGetAirlineResponse_GetUpdated_At
// Generated TestUpdateAirlineRequest_Reset
// Generated TestUpdateAirlineRequest_String
// Generated TestUpdateAirlineRequest_ProtoMessage
// Generated TestUpdateAirlineRequest_ProtoReflect
// Generated TestUpdateAirlineRequest_Descriptor
// Generated TestUpdateAirlineRequest_GetId
// Generated TestUpdateAirlineRequest_GetName
// Generated TestUpdateAirlineRequest_GetIataCode
// Generated TestUpdateAirlineRequest_GetIcaoCode
// Generated TestUpdateAirlineResponse_Reset
// Generated TestUpdateAirlineResponse_String
// Generated TestUpdateAirlineResponse_ProtoMessage
// Generated TestUpdateAirlineResponse_ProtoReflect
// Generated TestUpdateAirlineResponse_Descriptor
// Generated TestUpdateAirlineResponse_GetId
// Generated TestUpdateAirlineResponse_GetName
// Generated TestUpdateAirlineResponse_GetIataCode
// Generated TestUpdateAirlineResponse_GetIcaoCode
// Generated TestUpdateAirlineResponse_GetCountryCode
// Generated TestUpdateAirlineResponse_GetCreated_At
// Generated TestUpdateAirlineResponse_GetUpdated_At
// Generated TestDeleteAirlineRequest_Reset
// Generated TestDeleteAirlineRequest_String
// Generated TestDeleteAirlineRequest_ProtoMessage
// Generated TestDeleteAirlineRequest_ProtoReflect
// Generated TestDeleteAirlineRequest_Descriptor
// Generated TestDeleteAirlineRequest_GetId
// Generated TestDeleteAirlineRequest_GetName
// Generated TestDeleteAirlineRequest_GetIataCode
// Generated TestDeleteAirlineRequest_GetIcaoCode
// Generated TestDeleteAirlineResponse_Reset
// Generated TestDeleteAirlineResponse_String
// Generated TestDeleteAirlineResponse_ProtoMessage
// Generated TestDeleteAirlineResponse_ProtoReflect
// Generated TestDeleteAirlineResponse_Descriptor
// Generated TestDeleteAirlineResponse_GetId
// Generated TestDeleteAirlineResponse_GetName
// Generated TestDeleteAirlineResponse_GetIataCode
// Generated TestDeleteAirlineResponse_GetIcaoCode
// Generated TestDeleteAirlineResponse_GetCountryCode
// Generated TestDeleteAirlineResponse_GetUpdated_At
// Generated Test_file_rpc_airline_proto_rawDescGZIP
// Generated Test_file_rpc_airline_proto_init
// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        (unknown)
// source: rpc_airline.proto

package pb

import (
	reflect "reflect"
	"testing"

	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

func TestCreateAirlineRequest_Reset(t *testing.T) {
	type fields struct {
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &CreateAirlineRequest{
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
			}
			x.Reset()
		})
	}
}

func TestCreateAirlineRequest_String(t *testing.T) {
	type fields struct {
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &CreateAirlineRequest{
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
			}
			if got := x.String(); got != tt.want {
				t.Errorf("CreateAirlineRequest.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateAirlineRequest_ProtoMessage(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CreateAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
			}
			c.ProtoMessage()
		})
	}
}

func TestCreateAirlineRequest_ProtoReflect(t *testing.T) {
	type fields struct {
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
	}
	tests := []struct {
		name   string
		fields fields
		want   protoreflect.Message
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &CreateAirlineRequest{
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
			}
			if got := x.ProtoReflect(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateAirlineRequest.ProtoReflect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateAirlineRequest_Descriptor(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
	}
	tests := []struct {
		name   string
		fields fields
		want   []byte
		want1  []int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CreateAirlineRequest{
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
			}
			got, got1 := c.Descriptor()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateAirlineRequest.Descriptor() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("CreateAirlineRequest.Descriptor() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestCreateAirlineRequest_GetName(t *testing.T) {
	type fields struct {
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &CreateAirlineRequest{
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
			}
			if got := x.GetName(); got != tt.want {
				t.Errorf("CreateAirlineRequest.GetName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateAirlineRequest_GetIataCode(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &CreateAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
			}
			if got := x.GetIataCode(); got != tt.want {
				t.Errorf("CreateAirlineRequest.GetIataCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateAirlineRequest_GetIcaoCode(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &CreateAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
			}
			if got := x.GetIcaoCode(); got != tt.want {
				t.Errorf("CreateAirlineRequest.GetIcaoCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateAirlineRequest_GetCountryCode(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &CreateAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
			}
			if got := x.GetCountryCode(); got != tt.want {
				t.Errorf("CreateAirlineRequest.GetCountryCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLinks_Reset(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Website       *string
		Twitter       *string
		Instagram     *string
		Linkedin      *string
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &Links{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Website:       tt.fields.Website,
				Twitter:       tt.fields.Twitter,
				Instagram:     tt.fields.Instagram,
				Linkedin:      tt.fields.Linkedin,
			}
			x.Reset()
		})
	}
}

func TestLinks_String(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Website       *string
		Twitter       *string
		Instagram     *string
		Linkedin      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &Links{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Website:       tt.fields.Website,
				Twitter:       tt.fields.Twitter,
				Instagram:     tt.fields.Instagram,
				Linkedin:      tt.fields.Linkedin,
			}
			if got := x.String(); got != tt.want {
				t.Errorf("Links.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLinks_ProtoMessage(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Website       *string
		Twitter       *string
		Instagram     *string
		Linkedin      *string
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &Links{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Website:       tt.fields.Website,
				Twitter:       tt.fields.Twitter,
				Instagram:     tt.fields.Instagram,
				Linkedin:      tt.fields.Linkedin,
			}
			l.ProtoMessage()
		})
	}
}

func TestLinks_ProtoReflect(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Website       *string
		Twitter       *string
		Instagram     *string
		Linkedin      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   protoreflect.Message
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &Links{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Website:       tt.fields.Website,
				Twitter:       tt.fields.Twitter,
				Instagram:     tt.fields.Instagram,
				Linkedin:      tt.fields.Linkedin,
			}
			if got := x.ProtoReflect(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Links.ProtoReflect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLinks_Descriptor(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Website       *string
		Twitter       *string
		Instagram     *string
		Linkedin      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   []byte
		want1  []int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &Links{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Website:       tt.fields.Website,
				Twitter:       tt.fields.Twitter,
				Instagram:     tt.fields.Instagram,
				Linkedin:      tt.fields.Linkedin,
			}
			got, got1 := l.Descriptor()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Links.Descriptor() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("Links.Descriptor() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestLinks_GetWebsite(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Website       *string
		Twitter       *string
		Instagram     *string
		Linkedin      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &Links{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Website:       tt.fields.Website,
				Twitter:       tt.fields.Twitter,
				Instagram:     tt.fields.Instagram,
				Linkedin:      tt.fields.Linkedin,
			}
			if got := x.GetWebsite(); got != tt.want {
				t.Errorf("Links.GetWebsite() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLinks_GetTwitter(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Website       *string
		Twitter       *string
		Instagram     *string
		Linkedin      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &Links{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Website:       tt.fields.Website,
				Twitter:       tt.fields.Twitter,
				Instagram:     tt.fields.Instagram,
				Linkedin:      tt.fields.Linkedin,
			}
			if got := x.GetTwitter(); got != tt.want {
				t.Errorf("Links.GetTwitter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLinks_GetInstagram(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Website       *string
		Twitter       *string
		Instagram     *string
		Linkedin      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &Links{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Website:       tt.fields.Website,
				Twitter:       tt.fields.Twitter,
				Instagram:     tt.fields.Instagram,
				Linkedin:      tt.fields.Linkedin,
			}
			if got := x.GetInstagram(); got != tt.want {
				t.Errorf("Links.GetInstagram() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLinks_GetLinkedin(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Website       *string
		Twitter       *string
		Instagram     *string
		Linkedin      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &Links{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Website:       tt.fields.Website,
				Twitter:       tt.fields.Twitter,
				Instagram:     tt.fields.Instagram,
				Linkedin:      tt.fields.Linkedin,
			}
			if got := x.GetLinkedin(); got != tt.want {
				t.Errorf("Links.GetLinkedin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAirline_Reset(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &Airline{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			x.Reset()
		})
	}
}

func TestAirline_String(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &Airline{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.String(); got != tt.want {
				t.Errorf("Airline.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAirline_ProtoMessage(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Airline{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			a.ProtoMessage()
		})
	}
}

func TestAirline_ProtoReflect(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   protoreflect.Message
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &Airline{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.ProtoReflect(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Airline.ProtoReflect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAirline_Descriptor(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   []byte
		want1  []int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Airline{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			got, got1 := a.Descriptor()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Airline.Descriptor() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("Airline.Descriptor() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestAirline_GetId(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   int64
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &Airline{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetId(); got != tt.want {
				t.Errorf("Airline.GetId() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAirline_GetName(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &Airline{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetName(); got != tt.want {
				t.Errorf("Airline.GetName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAirline_GetIataCode(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &Airline{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetIataCode(); got != tt.want {
				t.Errorf("Airline.GetIataCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAirline_GetIcaoCode(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &Airline{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetIcaoCode(); got != tt.want {
				t.Errorf("Airline.GetIcaoCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAirline_GetCountryCode(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &Airline{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetCountryCode(); got != tt.want {
				t.Errorf("Airline.GetCountryCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAirline_GetCreated_At(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   *timestamppb.Timestamp
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &Airline{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetCreated_At(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Airline.GetCreated_At() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAirline_GetUpdated_At(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   *timestamppb.Timestamp
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &Airline{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetUpdated_At(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Airline.GetUpdated_At() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateAirlineResponse_Reset(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &CreateAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			x.Reset()
		})
	}
}

func TestCreateAirlineResponse_String(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &CreateAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.String(); got != tt.want {
				t.Errorf("CreateAirlineResponse.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateAirlineResponse_ProtoMessage(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CreateAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			c.ProtoMessage()
		})
	}
}

func TestCreateAirlineResponse_ProtoReflect(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   protoreflect.Message
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &CreateAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.ProtoReflect(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateAirlineResponse.ProtoReflect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateAirlineResponse_Descriptor(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   []byte
		want1  []int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CreateAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			got, got1 := c.Descriptor()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateAirlineResponse.Descriptor() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("CreateAirlineResponse.Descriptor() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestCreateAirlineResponse_GetId(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   int64
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &CreateAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetId(); got != tt.want {
				t.Errorf("CreateAirlineResponse.GetId() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateAirlineResponse_GetName(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &CreateAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetName(); got != tt.want {
				t.Errorf("CreateAirlineResponse.GetName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateAirlineResponse_GetIataCode(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &CreateAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetIataCode(); got != tt.want {
				t.Errorf("CreateAirlineResponse.GetIataCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateAirlineResponse_GetIcaoCode(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &CreateAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetIcaoCode(); got != tt.want {
				t.Errorf("CreateAirlineResponse.GetIcaoCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateAirlineResponse_GetCountryCode(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &CreateAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetCountryCode(); got != tt.want {
				t.Errorf("CreateAirlineResponse.GetCountryCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateAirlineResponse_GetCreated_At(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   *timestamppb.Timestamp
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &CreateAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetCreated_At(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateAirlineResponse.GetCreated_At() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateAirlineResponse_GetUpdated_At(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   *timestamppb.Timestamp
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &CreateAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetUpdated_At(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateAirlineResponse.GetUpdated_At() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestListAirlineRes_Reset(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Airlines      []*Airline
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &ListAirlineRes{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Airlines:      tt.fields.Airlines,
			}
			x.Reset()
		})
	}
}

func TestListAirlineRes_String(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Airlines      []*Airline
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &ListAirlineRes{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Airlines:      tt.fields.Airlines,
			}
			if got := x.String(); got != tt.want {
				t.Errorf("ListAirlineRes.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestListAirlineRes_ProtoMessage(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Airlines      []*Airline
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &ListAirlineRes{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Airlines:      tt.fields.Airlines,
			}
			l.ProtoMessage()
		})
	}
}

func TestListAirlineRes_ProtoReflect(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Airlines      []*Airline
	}
	tests := []struct {
		name   string
		fields fields
		want   protoreflect.Message
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &ListAirlineRes{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Airlines:      tt.fields.Airlines,
			}
			if got := x.ProtoReflect(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListAirlineRes.ProtoReflect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestListAirlineRes_Descriptor(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Airlines      []*Airline
	}
	tests := []struct {
		name   string
		fields fields
		want   []byte
		want1  []int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &ListAirlineRes{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Airlines:      tt.fields.Airlines,
			}
			got, got1 := l.Descriptor()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListAirlineRes.Descriptor() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("ListAirlineRes.Descriptor() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestListAirlineRes_GetAirlines(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Airlines      []*Airline
	}
	tests := []struct {
		name   string
		fields fields
		want   []*Airline
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &ListAirlineRes{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Airlines:      tt.fields.Airlines,
			}
			if got := x.GetAirlines(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListAirlineRes.GetAirlines() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestListAirlinesReq_Reset(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Limit         int64
		Offset        int64
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &ListAirlinesReq{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Limit:         tt.fields.Limit,
				Offset:        tt.fields.Offset,
			}
			x.Reset()
		})
	}
}

func TestListAirlinesReq_String(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Limit         int64
		Offset        int64
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &ListAirlinesReq{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Limit:         tt.fields.Limit,
				Offset:        tt.fields.Offset,
			}
			if got := x.String(); got != tt.want {
				t.Errorf("ListAirlinesReq.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestListAirlinesReq_ProtoMessage(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Limit         int64
		Offset        int64
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &ListAirlinesReq{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Limit:         tt.fields.Limit,
				Offset:        tt.fields.Offset,
			}
			l.ProtoMessage()
		})
	}
}

func TestListAirlinesReq_ProtoReflect(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Limit         int64
		Offset        int64
	}
	tests := []struct {
		name   string
		fields fields
		want   protoreflect.Message
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &ListAirlinesReq{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Limit:         tt.fields.Limit,
				Offset:        tt.fields.Offset,
			}
			if got := x.ProtoReflect(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListAirlinesReq.ProtoReflect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestListAirlinesReq_Descriptor(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Limit         int64
		Offset        int64
	}
	tests := []struct {
		name   string
		fields fields
		want   []byte
		want1  []int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &ListAirlinesReq{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Limit:         tt.fields.Limit,
				Offset:        tt.fields.Offset,
			}
			got, got1 := l.Descriptor()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListAirlinesReq.Descriptor() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("ListAirlinesReq.Descriptor() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestListAirlinesReq_GetLimit(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Limit         int64
		Offset        int64
	}
	tests := []struct {
		name   string
		fields fields
		want   int64
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &ListAirlinesReq{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Limit:         tt.fields.Limit,
				Offset:        tt.fields.Offset,
			}
			if got := x.GetLimit(); got != tt.want {
				t.Errorf("ListAirlinesReq.GetLimit() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestListAirlinesReq_GetOffset(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Limit         int64
		Offset        int64
	}
	tests := []struct {
		name   string
		fields fields
		want   int64
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &ListAirlinesReq{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Limit:         tt.fields.Limit,
				Offset:        tt.fields.Offset,
			}
			if got := x.GetOffset(); got != tt.want {
				t.Errorf("ListAirlinesReq.GetOffset() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAirlineRequest_Reset(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &GetAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			x.Reset()
		})
	}
}

func TestGetAirlineRequest_String(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &GetAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			if got := x.String(); got != tt.want {
				t.Errorf("GetAirlineRequest.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAirlineRequest_ProtoMessage(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GetAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			g.ProtoMessage()
		})
	}
}

func TestGetAirlineRequest_ProtoReflect(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   protoreflect.Message
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &GetAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			if got := x.ProtoReflect(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAirlineRequest.ProtoReflect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAirlineRequest_Descriptor(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   []byte
		want1  []int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GetAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			got, got1 := g.Descriptor()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAirlineRequest.Descriptor() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("GetAirlineRequest.Descriptor() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestGetAirlineRequest_GetId(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   int64
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &GetAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			if got := x.GetId(); got != tt.want {
				t.Errorf("GetAirlineRequest.GetId() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAirlineRequest_GetName(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &GetAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			if got := x.GetName(); got != tt.want {
				t.Errorf("GetAirlineRequest.GetName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAirlineRequest_GetIataCode(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &GetAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			if got := x.GetIataCode(); got != tt.want {
				t.Errorf("GetAirlineRequest.GetIataCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAirlineRequest_GetIcaoCode(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &GetAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			if got := x.GetIcaoCode(); got != tt.want {
				t.Errorf("GetAirlineRequest.GetIcaoCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAirlineResponse_Reset(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &GetAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			x.Reset()
		})
	}
}

func TestGetAirlineResponse_String(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &GetAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.String(); got != tt.want {
				t.Errorf("GetAirlineResponse.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAirlineResponse_ProtoMessage(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GetAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			g.ProtoMessage()
		})
	}
}

func TestGetAirlineResponse_ProtoReflect(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   protoreflect.Message
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &GetAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.ProtoReflect(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAirlineResponse.ProtoReflect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAirlineResponse_Descriptor(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   []byte
		want1  []int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GetAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			got, got1 := g.Descriptor()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAirlineResponse.Descriptor() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("GetAirlineResponse.Descriptor() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestGetAirlineResponse_GetId(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   int64
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &GetAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetId(); got != tt.want {
				t.Errorf("GetAirlineResponse.GetId() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAirlineResponse_GetName(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &GetAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetName(); got != tt.want {
				t.Errorf("GetAirlineResponse.GetName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAirlineResponse_GetIataCode(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &GetAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetIataCode(); got != tt.want {
				t.Errorf("GetAirlineResponse.GetIataCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAirlineResponse_GetIcaoCode(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &GetAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetIcaoCode(); got != tt.want {
				t.Errorf("GetAirlineResponse.GetIcaoCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAirlineResponse_GetCountryCode(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &GetAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetCountryCode(); got != tt.want {
				t.Errorf("GetAirlineResponse.GetCountryCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAirlineResponse_GetCreated_At(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   *timestamppb.Timestamp
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &GetAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetCreated_At(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAirlineResponse.GetCreated_At() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAirlineResponse_GetUpdated_At(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   *timestamppb.Timestamp
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &GetAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetUpdated_At(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAirlineResponse.GetUpdated_At() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateAirlineRequest_Reset(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            *int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &UpdateAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			x.Reset()
		})
	}
}

func TestUpdateAirlineRequest_String(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            *int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &UpdateAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			if got := x.String(); got != tt.want {
				t.Errorf("UpdateAirlineRequest.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateAirlineRequest_ProtoMessage(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            *int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &UpdateAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			u.ProtoMessage()
		})
	}
}

func TestUpdateAirlineRequest_ProtoReflect(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            *int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   protoreflect.Message
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &UpdateAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			if got := x.ProtoReflect(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UpdateAirlineRequest.ProtoReflect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateAirlineRequest_Descriptor(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            *int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   []byte
		want1  []int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &UpdateAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			got, got1 := u.Descriptor()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UpdateAirlineRequest.Descriptor() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("UpdateAirlineRequest.Descriptor() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestUpdateAirlineRequest_GetId(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            *int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   int64
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &UpdateAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			if got := x.GetId(); got != tt.want {
				t.Errorf("UpdateAirlineRequest.GetId() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateAirlineRequest_GetName(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            *int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &UpdateAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			if got := x.GetName(); got != tt.want {
				t.Errorf("UpdateAirlineRequest.GetName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateAirlineRequest_GetIataCode(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            *int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &UpdateAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			if got := x.GetIataCode(); got != tt.want {
				t.Errorf("UpdateAirlineRequest.GetIataCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateAirlineRequest_GetIcaoCode(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            *int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &UpdateAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			if got := x.GetIcaoCode(); got != tt.want {
				t.Errorf("UpdateAirlineRequest.GetIcaoCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateAirlineResponse_Reset(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &UpdateAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			x.Reset()
		})
	}
}

func TestUpdateAirlineResponse_String(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &UpdateAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.String(); got != tt.want {
				t.Errorf("UpdateAirlineResponse.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateAirlineResponse_ProtoMessage(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &UpdateAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			u.ProtoMessage()
		})
	}
}

func TestUpdateAirlineResponse_ProtoReflect(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   protoreflect.Message
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &UpdateAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.ProtoReflect(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UpdateAirlineResponse.ProtoReflect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateAirlineResponse_Descriptor(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   []byte
		want1  []int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &UpdateAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			got, got1 := u.Descriptor()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UpdateAirlineResponse.Descriptor() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("UpdateAirlineResponse.Descriptor() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestUpdateAirlineResponse_GetId(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   int64
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &UpdateAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetId(); got != tt.want {
				t.Errorf("UpdateAirlineResponse.GetId() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateAirlineResponse_GetName(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &UpdateAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetName(); got != tt.want {
				t.Errorf("UpdateAirlineResponse.GetName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateAirlineResponse_GetIataCode(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &UpdateAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetIataCode(); got != tt.want {
				t.Errorf("UpdateAirlineResponse.GetIataCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateAirlineResponse_GetIcaoCode(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &UpdateAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetIcaoCode(); got != tt.want {
				t.Errorf("UpdateAirlineResponse.GetIcaoCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateAirlineResponse_GetCountryCode(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &UpdateAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetCountryCode(); got != tt.want {
				t.Errorf("UpdateAirlineResponse.GetCountryCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateAirlineResponse_GetCreated_At(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   *timestamppb.Timestamp
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &UpdateAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetCreated_At(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UpdateAirlineResponse.GetCreated_At() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateAirlineResponse_GetUpdated_At(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Created_At    *timestamppb.Timestamp
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   *timestamppb.Timestamp
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &UpdateAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Created_At:    tt.fields.Created_At,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetUpdated_At(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UpdateAirlineResponse.GetUpdated_At() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteAirlineRequest_Reset(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            *int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &DeleteAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			x.Reset()
		})
	}
}

func TestDeleteAirlineRequest_String(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            *int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &DeleteAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			if got := x.String(); got != tt.want {
				t.Errorf("DeleteAirlineRequest.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteAirlineRequest_ProtoMessage(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            *int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DeleteAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			d.ProtoMessage()
		})
	}
}

func TestDeleteAirlineRequest_ProtoReflect(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            *int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   protoreflect.Message
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &DeleteAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			if got := x.ProtoReflect(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DeleteAirlineRequest.ProtoReflect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteAirlineRequest_Descriptor(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            *int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   []byte
		want1  []int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DeleteAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			got, got1 := d.Descriptor()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DeleteAirlineRequest.Descriptor() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("DeleteAirlineRequest.Descriptor() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestDeleteAirlineRequest_GetId(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            *int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   int64
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &DeleteAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			if got := x.GetId(); got != tt.want {
				t.Errorf("DeleteAirlineRequest.GetId() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteAirlineRequest_GetName(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            *int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &DeleteAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			if got := x.GetName(); got != tt.want {
				t.Errorf("DeleteAirlineRequest.GetName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteAirlineRequest_GetIataCode(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            *int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &DeleteAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			if got := x.GetIataCode(); got != tt.want {
				t.Errorf("DeleteAirlineRequest.GetIataCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteAirlineRequest_GetIcaoCode(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            *int64
		Name          *string
		IataCode      *string
		IcaoCode      *string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &DeleteAirlineRequest{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
			}
			if got := x.GetIcaoCode(); got != tt.want {
				t.Errorf("DeleteAirlineRequest.GetIcaoCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteAirlineResponse_Reset(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &DeleteAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Updated_At:    tt.fields.Updated_At,
			}
			x.Reset()
		})
	}
}

func TestDeleteAirlineResponse_String(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &DeleteAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.String(); got != tt.want {
				t.Errorf("DeleteAirlineResponse.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteAirlineResponse_ProtoMessage(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DeleteAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Updated_At:    tt.fields.Updated_At,
			}
			d.ProtoMessage()
		})
	}
}

func TestDeleteAirlineResponse_ProtoReflect(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   protoreflect.Message
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &DeleteAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.ProtoReflect(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DeleteAirlineResponse.ProtoReflect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteAirlineResponse_Descriptor(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   []byte
		want1  []int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DeleteAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Updated_At:    tt.fields.Updated_At,
			}
			got, got1 := d.Descriptor()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DeleteAirlineResponse.Descriptor() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("DeleteAirlineResponse.Descriptor() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestDeleteAirlineResponse_GetId(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   int64
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &DeleteAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetId(); got != tt.want {
				t.Errorf("DeleteAirlineResponse.GetId() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteAirlineResponse_GetName(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &DeleteAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetName(); got != tt.want {
				t.Errorf("DeleteAirlineResponse.GetName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteAirlineResponse_GetIataCode(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &DeleteAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetIataCode(); got != tt.want {
				t.Errorf("DeleteAirlineResponse.GetIataCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteAirlineResponse_GetIcaoCode(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &DeleteAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetIcaoCode(); got != tt.want {
				t.Errorf("DeleteAirlineResponse.GetIcaoCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteAirlineResponse_GetCountryCode(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &DeleteAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetCountryCode(); got != tt.want {
				t.Errorf("DeleteAirlineResponse.GetCountryCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteAirlineResponse_GetUpdated_At(t *testing.T) {
	type fields struct {
		state         protoimpl.MessageState
		sizeCache     protoimpl.SizeCache
		unknownFields protoimpl.UnknownFields
		Id            int64
		Name          string
		IataCode      string
		IcaoCode      string
		CountryCode   string
		Updated_At    *timestamppb.Timestamp
	}
	tests := []struct {
		name   string
		fields fields
		want   *timestamppb.Timestamp
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &DeleteAirlineResponse{
				state:         tt.fields.state,
				sizeCache:     tt.fields.sizeCache,
				unknownFields: tt.fields.unknownFields,
				Id:            tt.fields.Id,
				Name:          tt.fields.Name,
				IataCode:      tt.fields.IataCode,
				IcaoCode:      tt.fields.IcaoCode,
				CountryCode:   tt.fields.CountryCode,
				Updated_At:    tt.fields.Updated_At,
			}
			if got := x.GetUpdated_At(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DeleteAirlineResponse.GetUpdated_At() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_file_rpc_airline_proto_rawDescGZIP(t *testing.T) {
	tests := []struct {
		name string
		want []byte
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := file_rpc_airline_proto_rawDescGZIP(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("file_rpc_airline_proto_rawDescGZIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_file_rpc_airline_proto_init(t *testing.T) {
	tests := []struct {
		name string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file_rpc_airline_proto_init()
		})
	}
}
