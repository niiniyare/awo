package createairline

import (
	"context"
	"testing"

	"github.com/Abdirahman0022/airline/domain/entity"
	"github.com/Abdirahman0022/airline/infrastructure/log"
)

type mockOutportNormal struct {
	t *testing.T
}

// TestCaseNormal is for the case where ...
// explain the purpose of this test here with human readable naration...
func TestCaseNormal(t *testing.T) {

	ctx := context.Background()

	mockOutport := mockOutportNormal{
		t: t,
	}

	res, err := NewUsecase(&mockOutport).Execute(ctx, InportRequest{
		CompanyName: "Turkish Airlines",
		IataCode:    "TK",
	})

	if err != nil {
		t.Errorf("%v", err.Error())
		t.FailNow()
	}

	t.Logf("%v", res)

}

func (r *mockOutportNormal) SaveAirline(ctx context.Context, obj *entity.Airline) error {
	log.Info(ctx, "called")

	return nil
}
