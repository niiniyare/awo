package entity

import (
	"github.com/Abdirahman0022/airline/application/apperror"
	"strings"
	"time"
)

type Airline struct {
	ID          int64     `` //
	CompanyName string    `` //
	IataCode    string    `` //
	CreatedAt   time.Time `` //
}

type AirlineRequest struct {
	CompanyName string `` //
	IataCode    string `` //

}

func NewAirline(req AirlineRequest) (*Airline, error) {

	var obj Airline
	req.CompanyName = obj.CompanyName
	req.IataCode = obj.IataCode

	err := obj.Validate()
	if err != nil {
		return nil, err
	}

	return &obj, nil
}

func (r *Airline) Validate() error {
//	if r.ID == 0 {
//		return apperror.IDNotFound
//	}
	if strings.TrimSpace(r.CompanyName) == "" {
		return apperror.CompanyNameMustNotBeNotEmpty
	}
	if strings.TrimSpace(r.IataCode) == "" {
		return apperror.AirlineIataCodeMustNotBeNotEmpty
	}
	if len(r.IataCode) != 2 {
		return apperror.AirlineIataCodeMustBeTwoLetters
	}
	return nil
}
