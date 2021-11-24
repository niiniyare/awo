package createairline

import "github.com/Abdirahman0022/airline/domain/repository"

// Outport of CreateAirline
type Outport interface {
	repository.SaveAirlineRepo
}
